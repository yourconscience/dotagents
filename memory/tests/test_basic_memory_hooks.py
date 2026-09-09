from __future__ import annotations

import concurrent.futures
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

MEMORY_DIR = Path(__file__).resolve().parents[1]
END_HOOK = MEMORY_DIR / "hooks" / "basic-session-end.py"
START_HOOK = MEMORY_DIR / "hooks" / "basic-session-start.py"
DREAM_SCRIPT = MEMORY_DIR / "lib" / "basic_memory.py"
AMP_DIGEST = MEMORY_DIR / "lib" / "amp_digest.py"
FACTORY_DIGEST = MEMORY_DIR / "lib" / "factory_digest.py"
HERMES_DIGEST = MEMORY_DIR / "lib" / "hermes_digest.py"
SESSION_END_HOOK = MEMORY_DIR / "hooks" / "session-end.sh"
SESSION_START_HOOK = MEMORY_DIR / "hooks" / "session-start.sh"
STOP_HOOK = MEMORY_DIR / "hooks" / "stop.sh"


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

    def test_basic_digest_redacts_unlabelled_provider_tokens(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            secrets = [
                "sk-proj-abcdefghijklmnopqrstuvwx",
                "ghp_abcdefghijklmnopqrstuvwxyz123456",
                "github_pat_abcdefghijklmnopqrstuvwxyz123456",
                "glpat-" + "abcdefghijklmnopqrstuvwxyz123456",
                "xoxb-" + "123456789012-abcdefghijklmnop",
            ]
            payload = {
                "hook_event_name": "SessionEnd",
                "session_id": "bare-token-redaction",
                "session_start": "2026-07-16T03:04:05Z",
                "messages": [{"role": "user", "content": " ".join(secrets)}],
            }

            result = self.run_hook(END_HOOK, payload, env=self.env_with_knowledge(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            content = (knowledge / "sessions" / "2026-07-16.md").read_text(encoding="utf-8")
            for secret in secrets:
                self.assertNotIn(secret, content)
            self.assertIn("[REDACTED]", content)

    def test_provider_digests_persist_when_memsearch_binary_disappears(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            env = self.env_with_knowledge(knowledge, {"PATH": ""})

            factory_transcript = tmp_path / "factory-session.jsonl"
            factory_transcript.write_text(
                "\n".join(
                    [
                        json.dumps({"type": "session_start", "id": "factory-no-memsearch", "timestamp": "2026-07-16T05:06:07Z"}),
                        json.dumps(
                            {
                                "type": "message",
                                "timestamp": "2026-07-16T05:07:08Z",
                                "message": {"role": "user", "content": "Persist the Factory digest."},
                            }
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            factory = self.run_hook(
                FACTORY_DIGEST,
                {"session_id": "factory-no-memsearch", "transcript_path": str(factory_transcript)},
                env=env,
            )
            self.assertEqual(factory.returncode, 0, factory.stderr)
            self.assertTrue(json.loads(factory.stdout)["continue"])

            hermes_home = tmp_path / "hermes"
            hermes_sessions = hermes_home / "sessions"
            hermes_sessions.mkdir(parents=True)
            hermes_data = {
                "session_id": "hermes-no-memsearch",
                "session_start": "2026-07-16T06:07:08",
                "messages": [{"role": "user", "content": "Persist the Hermes digest."}],
            }
            (hermes_sessions / "session_hermes-no-memsearch.json").write_text(json.dumps(hermes_data), encoding="utf-8")
            hermes = self.run_hook(
                HERMES_DIGEST,
                {"session_id": "hermes-no-memsearch"},
                env={**env, "HERMES_HOME": str(hermes_home)},
            )
            self.assertEqual(hermes.returncode, 0, hermes.stderr)
            self.assertEqual(json.loads(hermes.stdout)["action"], "continue")

            content = (knowledge / "sessions" / "2026-07-16.md").read_text(encoding="utf-8")
            self.assertIn("Persist the Factory digest.", content)
            self.assertIn("Persist the Hermes digest.", content)

    def test_session_end_dispatch_cleans_temporary_payload(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            fake_bin = tmp_path / "bin"
            fake_bin.mkdir()
            for command in ("cat", "dirname", "mkdir", "mktemp", "rm"):
                resolved = shutil.which(command)
                self.assertIsNotNone(resolved)
                (fake_bin / command).symlink_to(resolved)
            (fake_bin / "python3").symlink_to(sys.executable)

            temp_dir = tmp_path / "tmp"
            temp_dir.mkdir()
            knowledge = tmp_path / "knowledge"
            env = self.env_with_knowledge(
                knowledge,
                {
                    "PATH": str(fake_bin),
                    "TMPDIR": str(temp_dir),
                },
            )
            payload = {
                "platform": "amp",
                "session_id": "cleanup-temp",
                "session_start": "2026-07-16T07:08:09Z",
                "messages": [{"role": "user", "content": "Clean the payload file."}],
            }
            result = subprocess.run(
                ["/bin/bash", str(SESSION_END_HOOK)],
                input=json.dumps(payload),
                text=True,
                capture_output=True,
                env=env,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["action"], "continue")
            self.assertEqual(list(temp_dir.iterdir()), [])
            session_note = knowledge / "sessions" / "2026-07-16.md"
            self.assertTrue(session_note.is_file())
            self.assertIn("Clean the payload file.", session_note.read_text(encoding="utf-8"))

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


class DreamMemoryReviewTests(unittest.TestCase):
    def env_with_knowledge(self, knowledge_dir: Path) -> dict[str, str]:
        env = os.environ.copy()
        env["KNOWLEDGE_DIR"] = str(knowledge_dir)
        return env

    def digest(self, session_id: str, captured: str, request: str, *, assistant: str = "Done.") -> str:
        return "\n".join(
            [
                f"<!-- basic-memory-session:{session_id}:start -->",
                f"## Session {captured} UTC - {session_id}",
                "",
                "- source: claude-code",
                f"- first request: {request}",
                f"- final assistant output: {assistant}",
                "",
                f"<!-- basic-memory-session:{session_id}:end -->",
                "",
            ]
        )

    def run_dream(self, knowledge: Path, output: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(DREAM_SCRIPT), "dream", "--output", str(output)],
            text=True,
            capture_output=True,
            env=self.env_with_knowledge(knowledge),
            check=False,
        )

    def test_dream_reports_exact_repeated_preferences_deterministically(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            reviews = knowledge / "reviews"
            sessions.mkdir(parents=True)
            (sessions / "2026-07-01.md").write_text(
                self.digest("one", "2026-07-01 10:00", "I prefer concise reports."),
                encoding="utf-8",
            )
            (sessions / "2026-07-02.md").write_text(
                self.digest("two", "2026-07-02 10:00", "I prefer concise reports!"),
                encoding="utf-8",
            )

            first_path = reviews / "first.json"
            second_path = reviews / "second.json"
            first = self.run_dream(knowledge, first_path)
            second = self.run_dream(knowledge, second_path)
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertEqual(first_path.read_bytes(), second_path.read_bytes())

            report = json.loads(first_path.read_text(encoding="utf-8"))
            self.assertEqual(report["mode"], "review-only")
            self.assertEqual(report["summary"]["repeated_preferences"], 1)
            candidate = report["candidates"][0]
            self.assertEqual(candidate["kind"], "repeated_preference")
            self.assertEqual([item["session_id"] for item in candidate["evidence"]], ["one", "two"])

    def test_dream_ignores_non_user_incomplete_and_non_exact_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            (sessions / "2026-07-01.md").write_text(
                "\n".join(
                    [
                        self.digest("one", "2026-07-01 10:00", "I prefer concise replies."),
                        self.digest("two", "2026-07-01 11:00", "I prefer short replies."),
                        self.digest("three", "2026-07-01 12:00", "I prefer concise replies…"),
                        "- first request: I prefer concise replies.",
                        "<!-- basic-memory-session:incomplete:start -->",
                        "## Session 2026-07-01 13:00 UTC - incomplete",
                        "- first request: I prefer concise replies.",
                    ]
                ),
                encoding="utf-8",
            )

            output = knowledge / "reviews" / "review.json"
            result = self.run_dream(knowledge, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["repeated_preferences"], 0)
            self.assertEqual(report["candidates"], [])

    def test_dream_finds_objective_duplicates_and_never_mutates_canonical_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            profile = knowledge / "profile"
            sessions.mkdir(parents=True)
            profile.mkdir()
            block = self.digest("duplicate", "2026-07-01 10:00", "Keep this record.")
            canonical = sessions / "2026-07-01.md"
            stale = sessions / "2026-07-02.md"
            canonical.write_text(block, encoding="utf-8")
            stale.write_text(block, encoding="utf-8")
            user = profile / "USER.md"
            user.write_text("- I prefer profile text to remain untouched.\n", encoding="utf-8")
            before = {path: path.read_bytes() for path in (canonical, stale, user)}

            output = knowledge / "reviews" / "review.json"
            result = self.run_dream(knowledge, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["stale_records"], 1)
            self.assertEqual(report["candidates"][0]["kind"], "stale_duplicate_record")
            self.assertEqual({path: path.read_bytes() for path in before}, before)

            replay = self.run_dream(knowledge, output)
            self.assertNotEqual(replay.returncode, 0)
            self.assertIn("already exists", replay.stderr)
            outside = self.run_dream(knowledge, Path(tmp) / "outside.json")
            self.assertNotEqual(outside.returncode, 0)
            self.assertIn("must be under", outside.stderr)


    def test_dream_reports_exact_legacy_sync_duplicates_without_guessing(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            (sessions / "knowledge.md").write_text(
                "\n".join(
                    [
                        "# Legacy export",
                        "",
                        "## Sync 2026-05-01 10:00 UTC",
                        "",
                        "- User prefers pnpm for Node work.",
                        "- Similar but distinct fact.",
                        "",
                        "## Sync 2026-05-02 10:00 UTC",
                        "",
                        "- User prefers pnpm for Node work.",
                        "- Similar but different fact.",
                        "",
                    ]
                ),
                encoding="utf-8",
            )

            output = knowledge / "reviews" / "review.json"
            result = self.run_dream(knowledge, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["summary"]["stale_records"], 1)
            candidate = report["candidates"][0]
            self.assertEqual(candidate["kind"], "stale_duplicate_record")
            self.assertEqual(candidate["occurrence_count"], 2)
            self.assertEqual(len(candidate["evidence"]), 2)

    def test_dream_truncates_legacy_evidence_beyond_twenty_occurrences(self):
        with tempfile.TemporaryDirectory() as tmp:
            knowledge = Path(tmp) / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            fact = "- User prefers pnpm for Node work."
            body = []
            for day in range(1, 22):  # 21 distinct sync sections repeat the same fact
                body += [f"## Sync 2026-05-{day:02d} 10:00 UTC", "", fact, ""]
            (sessions / "knowledge.md").write_text("\n".join(body) + "\n", encoding="utf-8")

            output = knowledge / "reviews" / "review.json"
            result = self.run_dream(knowledge, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            candidate = report["candidates"][0]
            self.assertEqual(candidate["kind"], "stale_duplicate_record")
            self.assertEqual(candidate["occurrence_count"], 21)
            self.assertEqual(len(candidate["evidence"]), 20)
            self.assertEqual(candidate["evidence_omitted"], 1)

class ClaudeFallbackAndDispatchTests(unittest.TestCase):
    """R1 fallback, R7 Codex/OMP capture, and R2 bounded reindex."""

    def run_shell(self, hook: Path, payload: object, *, env: dict[str, str], raw: str | None = None):
        stdin = raw if raw is not None else json.dumps(payload)
        return subprocess.run(
            ["/bin/bash", str(hook)],
            input=stdin,
            text=True,
            capture_output=True,
            env=env,
            check=False,
        )

    def base_env(self, knowledge: Path, extra: dict[str, str] | None = None) -> dict[str, str]:
        env = os.environ.copy()
        env["KNOWLEDGE_DIR"] = str(knowledge)
        env["MEMSEARCH_STATE_DIR"] = str(knowledge / "state")
        # A scratch collection keeps any stray reindex away from the real index.
        env["MEMSEARCH_COLLECTION"] = "test-scratch"
        # Point the plugin resolver at a directory that does not exist so the
        # Claude fallback is exercised (memsearch 0.2.x ships no claude-code plugin).
        env["MEMSEARCH_PLUGIN_DIR"] = str(knowledge / "no-such-plugin")
        if extra:
            env.update(extra)
        return env

    def fake_memsearch(self, fake_bin: Path, *, log: Path, sleep: float = 0.0, done_marker: Path | None = None) -> None:
        fake_bin.mkdir(parents=True, exist_ok=True)
        lines = ["#!/bin/sh", f'printf "%s\\n" "$*" >> "{log}"']
        if sleep:
            lines.append(f"sleep {sleep}")
        if done_marker is not None:
            lines.append(f'printf "done\\n" >> "{done_marker}"')
        lines.append("exit 0")
        script = fake_bin / "memsearch"
        script.write_text("\n".join(lines) + "\n", encoding="utf-8")
        script.chmod(script.stat().st_mode | stat.S_IXUSR)

    def with_fake_memsearch_path(self, env: dict[str, str], fake_bin: Path) -> dict[str, str]:
        env["PATH"] = str(fake_bin) + os.pathsep + env.get("PATH", "")
        return env

    def claude_payload(self, tmp_path: Path, session_id: str, first: str = "remember ripgrep over grep") -> dict:
        transcript = tmp_path / f"{session_id}.jsonl"
        transcript.write_text(
            "\n".join(
                [
                    json.dumps({"type": "session_start", "session_id": session_id, "timestamp": "2026-09-09T16:36:00Z"}),
                    json.dumps({"type": "message", "timestamp": "2026-09-09T16:36:01Z", "message": {"role": "user", "content": first}}),
                    json.dumps({"type": "message", "timestamp": "2026-09-09T16:36:05Z", "message": {"role": "assistant", "content": "Noted."}}),
                ]
            )
            + "\n",
            encoding="utf-8",
        )
        return {
            "hook_event_name": "SessionEnd",
            "session_id": session_id,
            "transcript_path": str(transcript),
            "model": "claude-opus-4-8",
        }

    def wait_for(self, predicate, timeout: float = 6.0, interval: float = 0.1) -> bool:
        import time

        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if predicate():
                return True
            time.sleep(interval)
        return predicate()

    def sole_digest(self, knowledge: Path) -> str:
        files = sorted((knowledge / "sessions").glob("*.md"))
        self.assertEqual(len(files), 1, files)
        return files[0].read_text(encoding="utf-8")

    def test_plugin_present_path_delegates_and_skips_basic_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            plugin_root = tmp_path / "plugin"
            (plugin_root / "hooks").mkdir(parents=True)
            plugin_marker = tmp_path / "plugin-ran"
            plugin_end = plugin_root / "hooks" / "session-end.sh"
            plugin_end.write_text(f'#!/bin/sh\nprintf "ran\\n" > "{plugin_marker}"\nexit 0\n', encoding="utf-8")
            plugin_end.chmod(plugin_end.stat().st_mode | stat.S_IXUSR)

            fake_bin = tmp_path / "bin"
            log = tmp_path / "memsearch.log"
            self.fake_memsearch(fake_bin, log=log)
            env = self.with_fake_memsearch_path(
                self.base_env(knowledge, {"MEMSEARCH_PLUGIN_DIR": str(plugin_root)}), fake_bin
            )

            result = self.run_shell(SESSION_END_HOOK, self.claude_payload(tmp_path, "plugin-present"), env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout), {"continue": True, "suppressOutput": True})
            self.assertTrue(plugin_marker.exists(), "plugin session-end.sh should run when the plugin resolves")
            self.assertEqual(list((knowledge / "sessions").glob("*.md")), [], "basic digest must not be written on the plugin path")

    def test_plugin_missing_fallback_writes_claude_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            env = self.base_env(knowledge)  # no memsearch on PATH -> reindex is a no-op

            result = self.run_shell(SESSION_END_HOOK, self.claude_payload(tmp_path, "claude-fallback"), env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            output = json.loads(result.stdout)
            self.assertTrue(output["continue"])
            self.assertIn("basic memory appended", output["systemMessage"])
            digest = self.sole_digest(knowledge)
            self.assertIn("basic-memory-session:claude-fallback:start", digest)
            self.assertIn("remember ripgrep over grep", digest)

    def test_plugin_missing_session_start_injects_context(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            sessions = knowledge / "sessions"
            sessions.mkdir(parents=True)
            (sessions / "2026-09-09.md").write_text("## Session digest\n- first request: prefer ripgrep\n", encoding="utf-8")

            result = self.run_shell(SESSION_START_HOOK, {"hook_event_name": "SessionStart"}, env=self.base_env(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            output = json.loads(result.stdout)
            self.assertEqual(output["hookSpecificOutput"]["hookEventName"], "SessionStart")
            self.assertIn("prefer ripgrep", output["hookSpecificOutput"]["additionalContext"])

    def test_codex_payload_classifies_to_basic_digest_labeled_codex(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            env = self.base_env(knowledge, {"DOTAGENTS_MEMORY_SOURCE": "codex"})
            payload = self.claude_payload(tmp_path, "codex-1", first="codex remember this")
            payload["model"] = "gpt-5-codex"

            result = self.run_shell(SESSION_END_HOOK, payload, env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("basic memory appended", json.loads(result.stdout)["systemMessage"])
            digest = self.sole_digest(knowledge)
            self.assertIn("- source: codex; model: gpt-5-codex", digest)
            self.assertIn("codex remember this", digest)

    def test_omp_payload_via_stop_hook_writes_basic_digest_labeled_omp(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            payload = {
                "hook_event_name": "Stop",
                "agent": "omp",
                "session_id": "omp-1",
                "messages": [{"role": "user", "content": "omp remember this"}],
            }
            result = self.run_shell(STOP_HOOK, payload, env=self.base_env(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("basic memory appended", json.loads(result.stdout)["systemMessage"])
            digest = self.sole_digest(knowledge)
            self.assertIn("- source: omp", digest)
            self.assertIn("omp remember this", digest)

    def test_stop_hook_claude_payload_does_not_capture(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            result = self.run_shell(STOP_HOOK, self.claude_payload(tmp_path, "claude-stop"), env=self.base_env(knowledge))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout), {"continue": True, "suppressOutput": True})
            self.assertFalse((knowledge / "sessions").exists(), "Claude Stop must not capture; SessionEnd owns the digest")

    def test_reindex_fires_after_append_and_is_gated_on_replay(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            fake_bin = tmp_path / "bin"
            log = tmp_path / "memsearch.log"
            self.fake_memsearch(fake_bin, log=log)
            env = self.with_fake_memsearch_path(self.base_env(knowledge), fake_bin)
            payload = self.claude_payload(tmp_path, "reindex-1")

            first = self.run_shell(SESSION_END_HOOK, payload, env=env)
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertTrue(self.wait_for(lambda: log.exists() and log.read_text().count("index") == 1), "one reindex expected after append")

            # Re-running the same session id is a replay -> no digest, no reindex.
            second = self.run_shell(SESSION_END_HOOK, payload, env=env)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertIn("skipped replayed", json.loads(second.stdout)["systemMessage"])
            import time

            time.sleep(0.8)
            self.assertEqual(log.read_text().count("index"), 1, "replay must not trigger a second reindex")

    def test_reindex_does_not_overlap_a_running_refresh(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            fake_bin = tmp_path / "bin"
            log = tmp_path / "memsearch.log"
            self.fake_memsearch(fake_bin, log=log)
            env = self.with_fake_memsearch_path(self.base_env(knowledge), fake_bin)
            # Simulate a refresh in flight by pre-holding the lock with a live owner.
            lock = knowledge / "state" / "reindex.lock"
            lock.mkdir(parents=True)
            (lock / "pid").write_text(str(os.getpid()), encoding="utf-8")

            result = self.run_shell(SESSION_END_HOOK, self.claude_payload(tmp_path, "no-overlap"), env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("basic memory appended", json.loads(result.stdout)["systemMessage"])
            import time

            time.sleep(0.8)
            self.assertFalse(log.exists(), "a live owner's lock must prevent an overlapping reindex")

    def test_reindex_reclaims_a_stale_lock_from_a_dead_owner(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            fake_bin = tmp_path / "bin"
            log = tmp_path / "memsearch.log"
            self.fake_memsearch(fake_bin, log=log)
            env = self.with_fake_memsearch_path(self.base_env(knowledge), fake_bin)
            # A lock left behind by a SIGKILLed refresher: its recorded owner is dead.
            dead = subprocess.Popen(["true"])
            dead.wait()
            lock = knowledge / "state" / "reindex.lock"
            lock.mkdir(parents=True)
            (lock / "pid").write_text(str(dead.pid), encoding="utf-8")

            result = self.run_shell(SESSION_END_HOOK, self.claude_payload(tmp_path, "stale-lock"), env=env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("basic memory appended", json.loads(result.stdout)["systemMessage"])
            self.assertTrue(
                self.wait_for(lambda: log.exists() and "index" in log.read_text()),
                "a stale lock from a dead owner must be reclaimed so reindex is not suppressed forever",
            )

    def test_reindex_is_nonblocking_and_bounded_by_watchdog(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            knowledge = tmp_path / "knowledge"
            fake_bin = tmp_path / "bin"
            log = tmp_path / "memsearch.log"
            done = tmp_path / "reindex-done"
            self.fake_memsearch(fake_bin, log=log, sleep=5, done_marker=done)
            env = self.with_fake_memsearch_path(
                self.base_env(knowledge, {"MEMSEARCH_REINDEX_TIMEOUT": "1"}), fake_bin
            )
            import time

            start = time.monotonic()
            result = self.run_shell(SESSION_END_HOOK, self.claude_payload(tmp_path, "bounded"), env=env)
            elapsed = time.monotonic() - start
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertLess(elapsed, 3.0, "hook must not block on the reindex")
            # The reindex started but the watchdog kills it before the 5s sleep completes.
            self.assertTrue(self.wait_for(lambda: log.exists()), "reindex should start")
            time.sleep(2.5)
            self.assertFalse(done.exists(), "watchdog must kill a reindex that exceeds the bound")
            self.assertFalse((knowledge / "state" / "reindex.lock").exists(), "lock must be released after the bounded run")


if __name__ == "__main__":
    unittest.main()
