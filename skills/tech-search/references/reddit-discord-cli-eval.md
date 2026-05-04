# Reddit and Discord CLI evaluation for `/tech-search`

Session context: evaluated `public-clis/rdt-cli` and `jackwener/discord-cli` as possible community-search backends for `/tech-search`.

## Recommendation

- Use `rdt-cli` as the preferred Reddit path when installed.
- Use `discord-cli` only as an opt-in backend for repeated Discord community monitoring or local archive search.
- Keep direct Discord API search as the default one-off path because it is simpler and avoids SQLite sync state.

## `public-clis/rdt-cli`

Repo: `https://github.com/public-clis/rdt-cli`

Findings:

- Python package: `rdt-cli`; install with `uv tool install rdt-cli`.
- Latest inspected CI was successful across Python 3.10, 3.12, and 3.13.
- Search supports global and subreddit-scoped queries with `--compact --json` / `--yaml`.
- Read supports post/comment fetches with structured JSON/YAML output; installed `rdt-cli==0.4.1` does not expose `--compact` for `rdt read`.
- Auth uses Reddit browser cookies or saved credentials under the user config directory; cookie material must never be printed or copied into chat.
- Source inspection found no obvious eval/exec/install-script red flags; subprocess usage was tied to browser-cookie extraction and browser opening helpers.
- Maintainer responsiveness looked good: compact/YAML/read bugs had been fixed quickly.

Good commands:

```bash
rdt search "<topic>" -s relevance -t month -n 10 --compact --json
rdt search "<topic>" -r <subreddit> -s top -t year -n 10 --compact --json
rdt read <post_id> -n 20 --json
```

Pitfalls:

- If auth/cookies fail, fall back to Reddit JSON with a browser User-Agent.
- For historical Reddit search, Pullpush remains a fallback.

## `jackwener/discord-cli`

Repo: `https://github.com/jackwener/discord-cli`

Findings:

- Python package: `kabi-discord-cli`; install with `uv tool install kabi-discord-cli`.
- Latest inspected CI was successful across Python 3.10, 3.12, and 3.14.
- Supports guild/channel discovery, native Discord search, channel sync, and local SQLite search/export.
- Uses `DISCORD_TOKEN` and includes helpers that can scan local browser/Discord client storage for user tokens.
- This creates material ToS/account-risk and secret-handling concerns. Do not ask the user to paste raw Discord tokens into chat logs. Never preserve token values.

Good commands:

```bash
discord status --yaml
discord dc guilds --yaml
discord dc channels <guild_id> --yaml
discord dc search <guild_id> "<topic>" -n 10 --json
discord search "<topic>" -n 20 --json
```

Pitfalls:

- User-token usage may violate Discord ToS or trigger account restrictions. Treat as opt-in.
- Local SQLite search requires prior sync; do not assume it has data.
- For one-off searches, direct Discord API search is usually simpler.
