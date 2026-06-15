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
- Viewing the playback queue
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
spotify_player playback play
spotify_player playback pause
spotify_player playback next
spotify_player playback previous
spotify_player playback shuffle
spotify_player playback repeat
spotify_player playback seek 30000              # seek forward 30s (offset in ms)
spotify_player playback volume 50               # set volume to 50%
spotify_player playback volume 10 --offset      # increase by 10%
spotify_player playback volume -10 --offset     # decrease by 10%
```

### Play by Name or ID

```bash
spotify_player playback start track --name "bohemian rhapsody"
spotify_player playback start track --id TRACKID
spotify_player playback start context album --name "kind of blue"
spotify_player playback start context playlist --name "chill vibes"
spotify_player playback start context playlist --id PLAYLISTID --shuffle
```

### Now Playing

```bash
spotify_player get key playback
spotify_player get key queue
```

### Search

```bash
spotify_player search "bohemian rhapsody"
spotify_player search "miles davis"
```

### Devices

```bash
spotify_player get key devices
spotify_player connect --name "Living Room Speaker"
spotify_player connect --id DEVICEID
```

### Playlists

```bash
spotify_player get key user-playlists
spotify_player playlist list
spotify_player playlist new "My Playlist"
spotify_player playlist new "Collab List" --public --collab
spotify_player playlist delete PLAYLISTID
spotify_player playlist edit add PLAYLISTID --track-id TRACKID
spotify_player playlist edit delete PLAYLISTID --track-id TRACKID
```

### Library

```bash
spotify_player get key user-liked-tracks
spotify_player get key user-saved-albums
spotify_player get key user-followed-artists
spotify_player get key user-top-tracks
```

## Troubleshooting

- **Auth failure on macOS Sequoia**: known issue with redirect URI. Try setting redirect URI to `http://127.0.0.1:8989/login` in your Spotify Developer app settings.
- **No playback devices**: open Spotify on any device first. `spotify_player` can control any Spotify Connect device remotely.
- **Socket connection refused**: a spotify_player instance may not be running. Commands will auto-start one, but it needs a few seconds to initialize.
