from __future__ import annotations

import concurrent.futures
import json
import os
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

MEMORY_DIR = Path(__file__).resolve().parents[1]
END_HOOK = MEMORY_DIR / "hooks" / "basic-session-end.py"
START_HOOK = MEMORY_DIR / "hooks" / "basic-session-start.py"
AMP_DIGEST = MEMORY_DIR / "lib" / "amp_digest.py"
HERMES_DIGEST = MEMORY_DIR / "lib" / "hermes_digest.py"


class BasicMemoryHookTests(unittest.TestCase):
    def run_hook(self, hook: Path, payload: object | None = None, *, env: dict[str, str], raw: str | None = None):
        stdin = raw if raw is not None else json.dumps(payload if payload is not None else {})
        return subprocess.run(
            [sys.executable, str(hook)],
            input=stdin,
            text=True,
            capture_output=True,
            env=env,
            check=False,
        )

    def env_with_knowledge(self, knowledge_dir: Path, extra: dict[str, str] | None = None) -> dict[str, str]:
        env = os.environ.copy()
        env["KNOWLEDGE_DIR"] = str(knowledge_dir)
        if extra:
            env.update(extra)
        return env

    def test_no_env_basic_first_run_uses_home_workspace_default(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp) / "home"
            env = os.environ.copy()
            env["HOME"] = str(home)
            env.pop("KNOWLEDGE_DIR", None)
            payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "first-run",
                "session_start": "2026-07-16T01:02:03Z",
                "messages": [{"role": "user", "content": "Remember the first run."}],
            }

            result = self.run_hook(END_HOOK, payload, env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            expected = home / "Workspace" / "knowledge" / "sessions" / "2026-07-16.md"
            output = json.loads(result.stdout)
            self.assertEqual(Path(output["output_file"]), expected.resolve())
            self.assertIn("Remember the first run.", expected.read_text(encoding="utf-8"))

    def test_env_override_uses_selected_knowledge_dir(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp) / "home"
            selected = Path(tmp) / "selected-vault"
            env = self.env_with_knowledge(selected, {"HOME": str(home)})
            payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "selected-vault",
                "session_start": "2026-07-16T02:03:04Z",
                "messages": [{"role": "user", "content": "Use the selected vault."}],
            }

            result = self.run_hook(END_HOOK, payload, env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            selected_file = selected / "sessions" / "2026-07-16.md"
            self.assertTrue(selected_file.exists())
            self.assertFalse((home / "Workspace" / "knowledge" / "sessions" / "2026-07-16.md").exists())

    def test_session_end_appends_to_dated_file_without_overwriting_and_skips_replay(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            target = sessions / "2026-07-14.md"
            target.write_text("existing entry\n", encoding="utf-8")
            raw_user_text = "Investigate ./memory and keep this long transcript detail private. " * 40
            payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "session-1",
                "session_start": "2026-07-14T10:11:12Z",
                "platform": "claude-code",
                "model": "test-model",
                "cwd": "/Users/example/project",
                "messages": [
                    {"role": "user", "content": raw_user_text, "timestamp": "2026-07-14T10:11:12Z"},
                    {"role": "assistant", "content": "Updated the focused memory hook implementation."},
                ],
            }

            first = self.run_hook(END_HOOK, payload, env=self.env_with_knowledge(knowledge))
            self.assertEqual(first.returncode, 0, first.stderr)
            first_stdout = json.loads(first.stdout)
            self.assertTrue(first_stdout["continue"])
            self.assertTrue(first_stdout["suppressOutput"])
            self.assertEqual(Path(first_stdout["output_file"]), target.resolve())

            content = target.read_text(encoding="utf-8")
            self.assertTrue(content.startswith("existing entry\n"), content)
            self.assertIn("<!-- basic-memory-session:session-1:start -->", content)
            self.assertIn("## Session 2026-07-14 10:11 UTC - session-1", content)
            self.assertIn("- first request: Investigate ./memory", content)
            self.assertIn("- final assistant output: Updated the focused memory hook implementation.", content)
            self.assertIn("`/Users/example/project`", content)
            self.assertNotIn(raw_user_text, content)

            before_replay = target.read_text(encoding="utf-8")
            replay = self.run_hook(END_HOOK, payload, env=self.env_with_knowledge(knowledge))
            self.assertEqual(replay.returncode, 0, replay.stderr)
            self.assertEqual(target.read_text(encoding="utf-8"), before_replay)
            replay_stdout = json.loads(replay.stdout)
            self.assertIn("skipped replayed session session-1", replay_stdout["systemMessage"])
            self.assertEqual(sorted(path.name for path in sessions.glob("*.md")), ["2026-07-14.md"])

    def test_session_end_reads_jsonl_transcript_and_uses_dated_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            transcript = tmp_path / "transcript.jsonl"
            transcript.write_text(
                "\n".join(
                    [
                        json.dumps({"timestamp": "2026-07-15T08:00:00Z", "session_id": "from-transcript", "type": "session_start"}),
                        json.dumps({"timestamp": "2026-07-15T08:01:00Z", "type": "message", "message": {"role": "user", "content": "Use ./memory/tests for coverage."}}),
                        json.dumps({"timestamp": "2026-07-15T08:02:00Z", "type": "message", "message": {"role": "assistant", "content": "Added behavior tests."}}),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            knowledge = tmp_path / "knowledge"
            payload = {"hook_event_name": "SessionEnd", "transcript_path": str(transcript), "platform": "claude-code"}

            result = self.run_hook(END_HOOK, payload, env=self.env_with_knowledge(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            output = json.loads(result.stdout)
            dated = knowledge / "sessions" / "2026-07-15.md"
            self.assertEqual(Path(output["output_file"]), dated.resolve())
            content = dated.read_text(encoding="utf-8")
            self.assertIn("<!-- basic-memory-session:from-transcript:start -->", content)
            self.assertIn("Use ./memory/tests for coverage.", content)
            self.assertIn("Added behavior tests.", content)

    def test_session_start_emits_bounded_deterministic_context(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            (sessions / "2026-07-13.md").write_text("older digest", encoding="utf-8")
            (sessions / "2026-07-15.md").write_text(("x" * 10000) + "\nlatest digest", encoding="utf-8")
            (sessions / "2026-07-14.markdown").write_text("middle digest", encoding="utf-8")

            result = self.run_hook(START_HOOK, {"hook_event_name": "SessionStart"}, env=self.env_with_knowledge(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            output = json.loads(result.stdout)
            self.assertTrue(output["continue"])
            self.assertTrue(output["suppressOutput"])
            hook_output = output["hookSpecificOutput"]
            self.assertEqual(hook_output["hookEventName"], "SessionStart")
            context = hook_output["additionalContext"]
            self.assertLessEqual(len(context), 6000)
            self.assertIn("2026-07-15.md", context)
            self.assertIn("latest digest", context)
            self.assertLess(context.index("2026-07-15.md"), context.index("2026-07-14.markdown"))

    def test_malformed_input_fails_actionably(self):
        with tempfile.TemporaryDirectory() as tmp:
            malformed = self.run_hook(END_HOOK, env=self.env_with_knowledge(Path(tmp) / "knowledge"), raw="{not json")
            self.assertNotEqual(malformed.returncode, 0)
            self.assertEqual(malformed.stdout, "")
            self.assertIn("invalid hook JSON", malformed.stderr)

    def test_amp_and_hermes_digests_redact_secrets_before_persisting(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            fake_bin = tmp_path / "bin"
            fake_bin.mkdir()
            fake_memsearch = fake_bin / "memsearch"
            fake_memsearch.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            fake_memsearch.chmod(fake_memsearch.stat().st_mode | stat.S_IXUSR)
            env = self.env_with_knowledge(knowledge, {"PATH": str(fake_bin)})

            amp_secret = "ampsecret123456789"
            path_secret = "pathsecret123456789"
            amp_payload = {
                "session_id": "amp-redact",
                "session_start": "2026-07-16T03:04:05Z",
                "model": f"model token={amp_secret}",
                "messages": [
                    {"role": "user", "content": f"Use api_key={amp_secret} in /Users/example/project?token={path_secret}"},
                    {"role": "assistant", "content": f"Done with Authorization: Bearer {amp_secret}"},
                ],
            }
            amp = self.run_hook(AMP_DIGEST, amp_payload, env=env)
            self.assertEqual(amp.returncode, 0, amp.stderr)

            hermes_home = tmp_path / "hermes"
            hermes_sessions = hermes_home / "sessions"
            hermes_sessions.mkdir(parents=True)
            hermes_secret = "hermessecret123456789"
            hermes_data = {
                "session_id": "hermes-redact",
                "session_start": "2026-07-16T04:05:06",
                "platform": f"hermes token={hermes_secret}",
                "model": f"model password={hermes_secret}",
                "messages": [
                    {"role": "user", "content": f"Open /Users/example/hermes?secret={hermes_secret}"},
                    {"role": "assistant", "content": f"Used Authorization: Bearer {hermes_secret}"},
                ],
            }
            (hermes_sessions / "session_hermes-redact.json").write_text(json.dumps(hermes_data), encoding="utf-8")
            hermes = self.run_hook(
                HERMES_DIGEST,
                {"session_id": "hermes-redact"},
                env={**env, "HERMES_HOME": str(hermes_home)},
            )
            self.assertEqual(hermes.returncode, 0, hermes.stderr)

            content = (knowledge / "sessions" / "2026-07-16.md").read_text(encoding="utf-8")
            self.assertNotIn(amp_secret, content)
            self.assertNotIn(path_secret, content)
            self.assertNotIn(hermes_secret, content)
            self.assertIn("[REDACTED]", content)

    def test_concurrent_same_and_different_session_writes_do_not_duplicate_or_drop(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            env = self.env_with_knowledge(knowledge)

            same_payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "same-session",
                "session_start": "2026-07-16T05:06:07Z",
                "messages": [{"role": "user", "content": "Only one digest should be kept."}],
            }
            with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
                same_results = list(pool.map(lambda _: self.run_hook(END_HOOK, same_payload, env=env), range(8)))
            for result in same_results:
                self.assertEqual(result.returncode, 0, result.stderr)

            target = knowledge / "sessions" / "2026-07-16.md"
            same_content = target.read_text(encoding="utf-8")
            self.assertEqual(same_content.count("<!-- basic-memory-session:same-session:start -->"), 1)

            different_payloads = [
                {
                    "hook_event_name": "SessionEnd",
                    "session_id": f"different-{index}",
                    "session_start": "2026-07-16T05:06:07Z",
                    "messages": [{"role": "user", "content": f"Digest {index} should persist."}],
                }
                for index in range(10)
            ]
            with concurrent.futures.ThreadPoolExecutor(max_workers=10) as pool:
                different_results = list(pool.map(lambda payload: self.run_hook(END_HOOK, payload, env=env), different_payloads))
            for result in different_results:
                self.assertEqual(result.returncode, 0, result.stderr)

            content = target.read_text(encoding="utf-8")
            for index in range(10):
                self.assertEqual(content.count(f"<!-- basic-memory-session:different-{index}:start -->"), 1)
                self.assertIn(f"Digest {index} should persist.", content)

    def test_basic_hooks_do_not_invoke_or_require_memsearch(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            fake_bin = tmp_path / "bin"
            fake_bin.mkdir()
            marker = tmp_path / "memsearch-was-called"
            fake_memsearch = fake_bin / "memsearch"
            fake_memsearch.write_text(f"#!/bin/sh\ntouch {marker}\nexit 99\n", encoding="utf-8")
            fake_memsearch.chmod(fake_memsearch.stat().st_mode | stat.S_IXUSR)

            knowledge = tmp_path / "knowledge"
            env = self.env_with_knowledge(knowledge, {"PATH": str(fake_bin)})
            payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "no-memsearch",
                "session_start": "2026-07-16T01:02:03Z",
                "messages": [{"role": "user", "content": "Remember the basic tier behavior."}],
            }

            end = self.run_hook(END_HOOK, payload, env=env)
            self.assertEqual(end.returncode, 0, end.stderr)
            start = self.run_hook(START_HOOK, {"hook_event_name": "SessionStart"}, env=env)
            self.assertEqual(start.returncode, 0, start.stderr)
            self.assertFalse(marker.exists(), "basic hooks invoked memsearch")


if __name__ == "__main__":
    unittest.main()
