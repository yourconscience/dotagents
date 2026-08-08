package dotagents

import "embed"

// StarterAssets contains the public files that dotagents setup scaffolds into a
// user-owned canonical configuration root. The CLI must not read these paths
// from the source checkout at runtime; release binaries carry them here.
//
//go:embed .gitignore AGENTS.md dotagents.yaml dotagents.lock plugin.json agents/*.md skills/dotagents skills/grilling memory/hooks memory/lib
var StarterAssets embed.FS
