import importlib.util
import os
import stat
import tempfile
import unittest
from pathlib import Path


SYNC_PATH = Path(__file__).parents[1] / "lib" / "sync.py"
SPEC = importlib.util.spec_from_file_location("dotagents_memory_sync", SYNC_PATH)
assert SPEC is not None
SYNC = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(SYNC)


class VaultToMemoryTests(unittest.TestCase):
    def test_full_user_memory_is_not_rewritten_or_reported_changed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            hermes_user = root / "hermes" / "USER.md"
            vault_profile = root / "vault" / "profile" / "USER.md"
            hermes_user.parent.mkdir(parents=True)
            vault_profile.parent.mkdir(parents=True)

            existing = "x" * (SYNC.HERMES_USER_LIMIT - 1) + "\n"
            hermes_user.write_text(existing, encoding="utf-8")
            vault_profile.write_text("# Profile\n\n- a distinct durable fact that cannot fit\n", encoding="utf-8")
            before_mtime = hermes_user.stat().st_mtime_ns

            changed = SYNC.vault_to_memory(
                {
                    "hermes_user": hermes_user,
                    "vault_profile": vault_profile,
                }
            )

            self.assertFalse(changed)
            self.assertEqual(hermes_user.read_text(encoding="utf-8"), existing)
            self.assertEqual(hermes_user.stat().st_mtime_ns, before_mtime)


class ReindexTests(unittest.TestCase):
    def _paths(self, root: Path) -> dict:
        return {
            "vault_dir": root / "vault",
            "memsearch_home": root / "memsearch_home",
            "collection": "ai",
        }

    def test_reindex_skips_cleanly_without_memsearch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            empty_bin = root / "emptybin"
            empty_bin.mkdir()
            old_path = os.environ.get("PATH", "")
            os.environ["PATH"] = str(empty_bin)
            try:
                SYNC.reindex_memsearch(self._paths(root))  # must not raise
            finally:
                os.environ["PATH"] = old_path

    def test_reindex_indexes_whole_vault_into_collection_ai(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake_bin = root / "bin"
            fake_bin.mkdir()
            argv_log = root / "argv.txt"
            fake = fake_bin / "memsearch"
            fake.write_text(
                "#!/usr/bin/env bash\n"
                f'printf "%s\\n" "$@" > "{argv_log}"\n'
                "exit 0\n",
                encoding="utf-8",
            )
            fake.chmod(fake.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)

            paths = self._paths(root)
            old_path = os.environ.get("PATH", "")
            os.environ["PATH"] = str(fake_bin) + os.pathsep + old_path
            try:
                SYNC.reindex_memsearch(paths)
            finally:
                os.environ["PATH"] = old_path

            argv = argv_log.read_text(encoding="utf-8").splitlines()
            self.assertEqual(argv[0], "index")
            self.assertEqual(argv[1], str(paths["vault_dir"]))
            self.assertIn("--collection", argv)
            self.assertEqual(argv[argv.index("--collection") + 1], "ai")
            # shared reindex lock was created under the engine home
            self.assertTrue((paths["memsearch_home"] / "reindex.lock").exists())

    def _fake_memsearch(self, root: Path) -> tuple[Path, Path]:
        fake_bin = root / "bin"
        fake_bin.mkdir()
        argv_log = root / "argv.txt"
        fake = fake_bin / "memsearch"
        fake.write_text(
            "#!/usr/bin/env bash\n"
            f'printf "%s\\n" "$@" > "{argv_log}"\n'
            "exit 0\n",
            encoding="utf-8",
        )
        fake.chmod(fake.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
        return fake_bin, argv_log

    def test_reindex_ignores_collection_drift_and_pins_ai(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake_bin, argv_log = self._fake_memsearch(root)
            paths = self._paths(root)
            paths["collection"] = "some-other-collection"  # drift must be ignored
            old_path = os.environ.get("PATH", "")
            os.environ["PATH"] = str(fake_bin) + os.pathsep + old_path
            try:
                SYNC.reindex_memsearch(paths)
            finally:
                os.environ["PATH"] = old_path
            argv = argv_log.read_text(encoding="utf-8").splitlines()
            self.assertEqual(argv[argv.index("--collection") + 1], "ai")

    def test_reindex_stays_best_effort_when_state_dir_unusable(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake_bin, argv_log = self._fake_memsearch(root)
            # A regular file where a directory is expected makes mkdir raise.
            blocker = root / "blocker"
            blocker.write_text("not a dir\n", encoding="utf-8")
            paths = self._paths(root)
            paths["memsearch_home"] = blocker / "nested"
            old_path = os.environ.get("PATH", "")
            os.environ["PATH"] = str(fake_bin) + os.pathsep + old_path
            try:
                SYNC.reindex_memsearch(paths)  # must not raise
            finally:
                os.environ["PATH"] = old_path
            # skipped before running memsearch
            self.assertFalse(argv_log.exists())


if __name__ == "__main__":
    unittest.main()
