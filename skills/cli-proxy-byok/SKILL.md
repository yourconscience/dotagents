---
name: cli-proxy-byok
description: Identify local cli-proxy/VibeProxy models that should be treated as BYOK models in Factory Droid.
---

# cli-proxy BYOK

Use this when the user asks whether Droid, VibeProxy, cli-proxy, Gemini, or Antigravity models are BYOK, or when classifying local proxy-backed model options.

## Rule

Treat models served through the local cli-proxy/VibeProxy OpenAI-compatible endpoint as BYOK when there is an enabled matching account in `~/.cli-proxy-api/`.

Local cli-proxy models are user-authenticated through local provider credentials, not Factory-hosted default models. Classify them by the model endpoint metadata plus enabled auth files, not by the model ID string alone.

## Discovery

1. Read `~/.cli-proxy-api/merged-config.yaml` for `host`, `port`, and `auth-dir`.
2. Query the OpenAI-compatible model surface:

```bash
curl -s "http://127.0.0.1:8318/v1/models"
```

3. Inspect only metadata from `~/.cli-proxy-api/*.json`:
   - `type`
   - `email`
   - `disabled`
   - `project_id`
   - expiry fields

Never print, copy, or summarize token fields such as `access_token`, `refresh_token`, `id_token`, `client_secret`, or nested `token` values.

Do not query `/v0/management/*` endpoints unless the user intentionally provides the management key and asks for management API inspection.

## BYOK classification

If an enabled `gemini-*.json` auth file exists, classify models from `/v1/models` with:

```text
owned_by: google
```

as Gemini BYOK models.

If an enabled `antigravity-*.json` auth file exists, classify models from `/v1/models` with:

```text
owned_by: antigravity
```

as Antigravity BYOK models.

The `id` can be misleading. For example, a `claude-*` or `gpt-*` model with `owned_by: antigravity` is Antigravity BYOK, while a `gemini-*` model with `owned_by: google` is Gemini BYOK.

## Current expected BYOK families

Gemini BYOK examples observed from cli-proxy:

```text
gemini-2.5-flash
gemini-2.5-flash-lite
gemini-2.5-pro
gemini-3-flash-preview
gemini-3-pro-preview
gemini-3.1-flash-lite-preview
gemini-3.1-pro-preview
```

Antigravity BYOK examples observed from cli-proxy:

```text
claude-opus-4-6-thinking
claude-sonnet-4-6
gemini-3-flash
gemini-3-flash-agent
gemini-3-pro-high
gemini-3-pro-low
gemini-3.1-flash-image
gemini-3.1-flash-lite
gemini-3.1-pro-low
gemini-3.5-flash-low
gemini-pro-agent
gpt-oss-120b-medium
```

Prefer the live `/v1/models` result over this static list when they differ.

## Reporting

When summarizing to the user, group local cli-proxy models like this:

```text
Gemini BYOK: <owned_by=google models with enabled gemini auth>
Antigravity BYOK: <owned_by=antigravity models with enabled antigravity auth>
Other proxy models: <all other owned_by values>
```

If an auth file exists but `disabled: true`, report the provider as signed in but disabled, not active BYOK.
