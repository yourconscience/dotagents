# Auth And Debugging

## Auth Model

- `x-cli auth login` launches Chrome and captures the cookies plus bearer and CSRF tokens needed for X's internal GraphQL endpoints.
- Credentials are stored at `~/.x-cli/credentials.json`.
- If login state becomes invalid, refresh it with `x-cli auth login` rather than editing the credentials file.

## Pagination And Rate Limits

- Timeline, search, followers, and following commands can return a cursor for the next page.
- `timeline` commands also support `--all` with `--max-pages`.
- The CLI parses X rate-limit headers and waits before retrying when required, so repeated requests may pause instead of failing immediately.

## Endpoint Drift

- X rotates GraphQL query IDs regularly.
- If several commands begin returning `404` responses at the same time, check `internal/api/endpoints.go`.
- Updated query IDs can usually be recovered from X's current production bundle or from captured browser network traffic.

## Useful Debug Path

```bash
x-cli auth status
x-cli timeline home --verbose
x-cli search "test" --type latest --json --verbose
```
