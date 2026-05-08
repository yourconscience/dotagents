package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyMemoryHookPaths(t *testing.T) {
	home := t.TempDir()
	agents := filepath.Join(home, ".agents")
	files := map[string]string{
		filepath.Join(home, ".claude", "settings.json"): `{
  "hooks": {
    "SessionStart": [{"hooks": [{"command": "bash ` + filepath.Join(agents, "bin", "memsearch", "hook.sh") + ` session-start"}]}],
    "Stop": [{"hooks": [{"command": "bash ` + filepath.Join(agents, "bin", "memsearch", "hook.sh") + ` stop"}]}],
    "SessionEnd": [{"hooks": [{"command": "bash ` + filepath.Join(agents, "bin", "memsearch", "hook.sh") + ` session-end"}]}]
  }
}`,
		filepath.Join(home, ".factory", "settings.json"): `{
  "hooks": {
    "SessionEnd": [{"hooks": [{"command": "` + filepath.Join(agents, "bin", "memsearch", "finalize.sh") + `"}]}]
  }
}`,
		filepath.Join(home, ".hermes", "config.yaml"): `hooks:
  on_session_finalize:
  - command: ~/.agents/bin/memsearch/finalize.sh
  - command: ~/.agents/bin/memsearch/sync-memory-to-vault.sh
  on_session_start:
  - command: ~/.agents/bin/memsearch/sync-vault-to-memory.sh
`,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	migrated, err := migrateLegacyMemoryHookPaths(home)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != 3 {
		t.Fatalf("migrated = %d, want 3", migrated)
	}

	for path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "bin/memsearch") {
			t.Fatalf("%s still contains legacy path:\n%s", path, text)
		}
		if !strings.Contains(text, "memory/hooks") {
			t.Fatalf("%s missing new memory hook path:\n%s", path, text)
		}
	}
}
