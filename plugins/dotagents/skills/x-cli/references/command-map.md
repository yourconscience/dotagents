# Command Map

## Auth

```bash
x-cli auth login
x-cli auth status
x-cli auth logout
```

## Timeline

```bash
x-cli timeline home [--count N] [--cursor CURSOR] [--all] [--max-pages N]
x-cli timeline user @handle [--count N] [--cursor CURSOR] [--all] [--max-pages N]
```

## Tweet

```bash
x-cli tweet get <tweet_id_or_url>
```

## User

```bash
x-cli user get @handle
```

## Search

```bash
x-cli search "<query>" [--type top|latest|people|media] [--count N] [--cursor CURSOR]
```

## Social Graph

```bash
x-cli followers @handle [--count N] [--cursor CURSOR]
x-cli following @handle [--count N] [--cursor CURSOR]
```

## Global Flags

```bash
--json
--verbose
```

Use `--json` for structured output and `--verbose` to debug request failures.
