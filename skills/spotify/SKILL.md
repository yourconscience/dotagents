---
name: spotify
description: Use this skill to control Spotify playback, search music, manage playlists, and queue tracks via the spotify_player CLI.
license: MIT
metadata:
  author: dotagents
  version: 1.0.0
  category: media
  tags: "spotify,music,playback,playlists"
---

# Spotify

Control Spotify playback via `spotify_player` CLI.

## Use This Skill For

- Playing, pausing, skipping tracks
- Searching for songs, albums, artists, playlists
- Managing the playback queue
- Listing and selecting playback devices
- Creating and modifying playlists
- Checking what is currently playing

## Prerequisites

- Spotify Premium account (required by Spotify API for playback control)
- `spotify_player` installed: `brew install spotify_player` or `cargo install spotify_player --locked`
- One-time auth: register a Spotify Developer app at developer.spotify.com, set `client_id` in `~/.config/spotify-player/app.toml`, then run `spotify_player authenticate`

## Core Rules

- Use `--json` or pipe through `jq` when output will be consumed by another tool or agent.
- Do not start the TUI unless the user explicitly asks for it. Use CLI subcommands for all agent operations.
- The CLI communicates via a local socket (default port 8080). If no instance is running, commands start a temporary client automatically.
- Treat auth tokens as managed state. Use `spotify_player authenticate` instead of editing config files manually.

## Commands

### Playback

```bash
spotify_player playback play-pause
spotify_player playback next
spotify_player playback previous
spotify_player playback play --name "track or album name"
spotify_player playback start --uri spotify:track:TRACKID
spotify_player playback shuffle
spotify_player playback repeat
spotify_player playback volume --offset 10
spotify_player playback volume --volume 50
```

### Now Playing

```bash
spotify_player get key playback
```

### Search

```bash
spotify_player search --query "bohemian rhapsody" --type track
spotify_player search --query "miles davis" --type artist
spotify_player search --query "chill vibes" --type playlist
```

### Queue

```bash
spotify_player queue --uri spotify:track:TRACKID
```

### Devices

```bash
spotify_player get key devices
spotify_player playback transfer --device "device name"
```

### Playlists

```bash
spotify_player get key playlists
spotify_player playlist create --name "My Playlist"
spotify_player playlist delete --id PLAYLISTID
```

### Library

```bash
spotify_player get key tracks
spotify_player get key albums
```

## Troubleshooting

- **Auth failure on macOS Sequoia**: known issue with redirect URI. Try setting redirect URI to `http://127.0.0.1:8989/login` in your Spotify Developer app settings.
- **No playback devices**: open Spotify on any device first. `spotify_player` can control any Spotify Connect device remotely.
- **Socket connection refused**: a spotify_player instance may not be running. Commands will auto-start one, but it needs a few seconds to initialize.
