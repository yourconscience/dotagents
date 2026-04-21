# portals-scan

Zero-token ATS portal scanner. Queries Greenhouse, Ashby, and Lever public APIs, filters by title keywords, and deduplicates against an existing opportunities tracker.

## Build

```bash
go build -o portals-scan .
```

## Usage

```bash
# Run from skill root (skills/jobs/) - finds data/portals.yml automatically
cd skills/jobs
go run ./tools/portals-scan

# Or use the built binary
./tools/portals-scan/portals-scan

# Scan a single company
go run ./tools/portals-scan --company Recraft

# Preview without writing
go run ./tools/portals-scan --dry-run

# JSON output for piping
go run ./tools/portals-scan --json

# Explicit paths
go run ./tools/portals-scan --config path/to/portals.yml --tracker path/to/opportunities.yaml
```

## Config

`portals.yml` defines tracked companies and title filters:

```yaml
title_filter:
  positive: ["ml", "machine learning", "ai engineer"]
  negative: ["intern", "manager", "frontend"]

tracked_companies:
  - name: Anthropic
    careers_url: https://job-boards.greenhouse.io/anthropic
    enabled: true
```

Supported platforms (auto-detected from `careers_url`):
- **Greenhouse**: `job-boards.greenhouse.io/{slug}`
- **Ashby**: `jobs.ashbyhq.com/{slug}`
- **Lever**: `jobs.lever.co/{slug}`

Companies with custom career portals (no supported ATS) should be set to `enabled: false`.
