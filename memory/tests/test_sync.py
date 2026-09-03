import importlib.util
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


if __name__ == "__main__":
    unittest.main()
