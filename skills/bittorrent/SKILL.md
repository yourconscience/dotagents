---
name: bittorrent
description: Use when managing BitTorrent downloads, magnet links, torrent metadata, tracker/DHT diagnostics, local client automation, or safe/legal torrent workflows.
---

# bittorrent

Use this skill for legitimate BitTorrent work: inspecting magnet links or
`.torrent` files, managing local downloads, diagnosing tracker/DHT/peer issues,
and automating a local BitTorrent client.

## Rules

- Work only with legal content the user has the right to download or seed.
- Do not start a download, seed, delete data, or expose a client Web UI without explicit user approval.
- Prefer existing local clients and CLIs over writing protocol code.
- Keep credentials, passkeys, private tracker URLs, and Web UI tokens out of logs and reports.
- Measure speeds, peer counts, ratios, and disk usage from the client; do not guess.

## Discovery

1. Identify the client and control surface:
   - Transmission: `transmission-remote`
   - qBittorrent: Web API, `qbt`, or existing local scripts
   - aria2: `aria2c` / JSON-RPC
   - Deluge: `deluge-console`
2. Check whether the task is metadata-only or will transfer data.
3. For transfer tasks, confirm destination path, disk space, and whether seeding is expected.
4. For private trackers, preserve the full announce URL but redact credentials in any output.

## Common Workflows

### Inspect a magnet link

Use metadata-only handling when possible. Extract and report:

- `btih` info hash
- display name (`dn`)
- trackers (`tr`)
- web seeds (`ws`)
- peer discovery flags implied by the client

Do not add the magnet to a client unless the user approves starting the job.

### Inspect a `.torrent` file

Read the bencoded metadata with an existing parser or client command. Report:

- name, total size, file count
- piece length and piece count
- private flag
- announce URL and announce-list tiers, with secrets redacted
- info hash if the tool provides it

### Add or manage a download

Before adding:

1. Confirm content legality and target directory.
2. Check free disk space.
3. Prefer paused add when the client supports it, then let the user approve start.

After adding, report client task ID/hash, save path, state, progress, peers,
download/upload speed, ratio, and ETA.

### Diagnose slow or stalled torrents

Check in this order:

1. Client state: paused, queued, errored, disk full, or path missing.
2. Torrent health: seed/peer count, availability, private flag.
3. Tracker status: last error, next announce, HTTP/TLS failures.
4. DHT/PEX/LSD settings for public torrents.
5. Network reachability: listening port open, firewall/NAT, VPN routing.
6. Ratio or share-limit rules that may stop seeding.

## Output Shape

For reports, keep the answer compact:

```markdown
## Status
- Client:
- Torrent:
- State:
- Progress:
- Peers/seeds:
- Speeds:
- Ratio:
- Path:

## Findings
- ...

## Next Action
- ...
```

## Common Mistakes

- Starting a torrent just to inspect metadata.
- Printing private tracker passkeys or authenticated Web UI URLs.
- Treating a tracker error as proof the torrent is dead before checking peers/DHT.
- Changing global client settings when a per-torrent setting is enough.
- Deleting torrent data when the user only asked to remove the task from the client.
