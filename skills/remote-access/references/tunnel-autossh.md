# Remote-access reverse tunnel hardening

Use this when the Mac bridge works locally on `127.0.0.1:18777` but VPS calls to `127.0.0.1:18778` intermittently print SSH channel errors such as `channel N: open failed: connect failed: open failed`.

## Layout

- Mac bridge LaunchAgent: `~/Library/LaunchAgents/com.conscience.remote-access.bridge.plist`
- Mac bridge binary: `~/.local/bin/remote-access-bridge`
- Bridge source: `~/Workspace/dotagents/skills/remote-access/tools/remote-access-bridge/`
- Tunnel LaunchAgent: `~/Library/LaunchAgents/com.conscience.remote-access.tunnel.plist`
- Local bridge endpoint: `http://127.0.0.1:18777`
- VPS reverse-forward endpoint: `http://127.0.0.1:18778`

## Diagnosis

Check both ends before changing tunnel machinery:

```bash
launchctl list | grep 'com.conscience.remote-access'
lsof -nP -iTCP:18777 -sTCP:LISTEN
ps -axo pid,ppid,etime,command | grep -E 'ssh .*18778|autossh' | grep -v grep
ssh vps 'curl -fsS --max-time 5 http://127.0.0.1:18778/status'
```

Interpretation:
- `18777` not listening: bridge LaunchAgent or binary is the problem.
- `18777` listening but VPS `18778` fails: reverse tunnel is stale or down.
- Existing interactive SSH shells may still print stale channel errors after the tunnel is repaired; reopen the shell if needed.

## Autossh migration

Install autossh on the Mac if needed:

```bash
brew install autossh
```

Change only the tunnel LaunchAgent. Keep the bridge LaunchAgent separate.

Recommended `ProgramArguments` for `com.conscience.remote-access.tunnel.plist`:

```xml
<array>
  <string>/opt/homebrew/bin/autossh</string>
  <string>-M</string>
  <string>0</string>
  <string>-N</string>
  <string>-o</string>
  <string>ExitOnForwardFailure=yes</string>
  <string>-o</string>
  <string>ServerAliveInterval=30</string>
  <string>-o</string>
  <string>ServerAliveCountMax=2</string>
  <string>-o</string>
  <string>TCPKeepAlive=yes</string>
  <string>-R</string>
  <string>127.0.0.1:18778:127.0.0.1:18777</string>
  <string>vps</string>
</array>
```

`-M 0` disables autossh's extra monitor port and relies on SSH keepalives. That is simpler for a Tailscale reverse tunnel.

Reload:

```bash
launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/com.conscience.remote-access.tunnel.plist
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.conscience.remote-access.tunnel.plist
launchctl kickstart -k "gui/$(id -u)/com.conscience.remote-access.tunnel"
```

Verify:

```bash
ps -axo pid,etime,command | grep '[a]utossh'
ssh vps 'curl -fsS http://127.0.0.1:18778/status'
```

## Pitfall

Autossh restarts dead SSH sessions, but it does not make a dead local bridge healthy. If the bridge process on `127.0.0.1:18777` is down, autossh can keep the tunnel process alive while every VPS request still fails with channel-open errors. Always check the bridge listener and `/status` endpoint too.
