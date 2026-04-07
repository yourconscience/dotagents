# X search workspace

This directory holds local, non-git state for `techsearch` X automation.

## Files

- `accounts.db` - `twscrape` account database
- `accounts.txt` - optional input file for `twscrape add_accounts`
- `playwright/.auth/user.json` - optional saved Playwright auth state for browser fallback

These local auth files are gitignored.

## Primary path

Run the helper from the shared skill repo:

```bash
uv run --with twscrape python ~/.agents/skills/techsearch/tools/x_search.py doctor
uv run --with twscrape python ~/.agents/skills/techsearch/tools/x_search.py search --query 'from:karpathy evals' --limit 5 --product Latest
```

## Account setup

Prefer cookie-backed accounts over repeated scripted login.

You should not need to log in on every run. `twscrape` persists session state in `accounts.db` and will reuse it until X invalidates the session or forces a fresh login challenge.

Practical expectation:
- normal runs should reuse the saved session
- occasional manual re-auth may still be needed after X invalidates cookies or flags the account
- keep Playwright auth as a backup path when `twscrape` login or GraphQL behavior changes

Example import command:

```bash
twscrape --db ~/Workspace/dotagents/skills/techsearch/x/accounts.db add_accounts ~/Workspace/dotagents/skills/techsearch/x/accounts.txt username:password:email:email_password:_:cookies
```

## Browser fallback

If `twscrape` breaks, use Playwright with a saved login state in:

```text
~/Workspace/dotagents/skills/techsearch/x/playwright/.auth/user.json
```

Do not commit anything from this directory.
