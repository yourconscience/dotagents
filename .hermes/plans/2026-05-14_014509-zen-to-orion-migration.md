# Zen to Orion migration plan

Generated: 2026-05-14 01:45 +04

## Goal

Migrate the user's Zen setup to Orion with minimum risk:

- pinned / essential tabs
- workspace/profile mental model
- bookmarks
- extensions
- adblock/custom filters
- visual browser layout and sidebar feel

Orion was described as just downloaded and safe to modify, but inspection shows Orion already has a small live profile, so back it up before any mutation.

## Current findings

### Hermes computer-use status

Computer-use is not active in the current Hermes setup.

Evidence:

- `hermes tools list` reports: `✗ disabled computer_use 🖱️ Computer Use (macOS)`
- Current session tool schema does not include a `computer_use` tool namespace, so I cannot visually drive/capture Zen or Orion from this session.
- `hermes plugins list` shows no computer-use plugin entry. This looks like a core toolset, not a plugin toggle.
- `~/.hermes/config.yaml` `platform_toolsets` does not include `computer_use` for `cli`, `telegram`, or `discord`.
- `command -v cua-driver` and `command -v cua` returned no path, so the macOS driver binary is probably not installed or not on PATH.

Likely enablement path:

1. Run `hermes tools enable computer_use` or use interactive `hermes tools`.
2. Grant macOS Accessibility and Screen Recording permissions if prompted.
3. Restart the Hermes session or gateway. Tool changes do not apply mid-conversation.
4. Verify with `hermes tools list` and a new session that exposes `computer_use`.
5. First test should be non-destructive: `capture` Zen and Orion, no clicks.

### Zen state

Zen app exists at:

- `/Applications/Zen.app`

Zen profile root:

- `/Users/conscience/Library/Application Support/zen`

Profiles:

- Main active profile: `/Users/conscience/Library/Application Support/zen/Profiles/p4opb071.Default (release)`
- Empty/inactive-looking profile: `/Users/conscience/Library/Application Support/zen/Profiles/q59nt465.Default Profile`

Zen is currently running with many processes. The active profile is `p4opb071.Default (release)`.

Important files in the active profile:

- `places.sqlite`, `favicons.sqlite`: bookmarks/history/favicons
- `extensions.json`: extension inventory
- `containers.json`: Firefox container identities
- `prefs.js`, `xulstore.json`: settings/UI state
- `zen-sessions.jsonlz4`: Zen workspace/tab state
- `zen-sessions-backup/clean.jsonlz4`: backup copy of Zen tab state
- `sessionstore-backups/recovery.jsonlz4`: Firefox session recovery data
- `zen-live-folders.jsonlz4`: currently empty

Read-only bookmark inspection:

- bookmarks: 907
- folders: 140
- places/history rows: 6250

Zen containers:

- Personal, userContextId 1
- Work, userContextId 2
- Banking, userContextId 3
- Shopping, userContextId 4

Zen spaces/workspaces from `zen-sessions.jsonlz4`:

- `lab`, icon 💻️
- `joy`, icon 🌈
- 6 essential tabs have no workspace id but appear to map to the personal/default space behavior

Zen tab/session summary:

- `zen-sessions.jsonlz4`: 113 tab records, 527 unique historical/session URLs
- Workspace `lab`: 99 tabs, 10 pinned, 6 essential
- Workspace `joy`: 8 tabs, 5 pinned
- No-workspace essential set: 6 pinned/essential tabs
- Top domains in active session: GitHub, Google, Google Translate, YouTube, LinkedIn, X, ChatGPT, Claude, Instagram, SoundCloud, local Hermes dashboard

Visible active Zen extensions:

- uBlock Origin
- SponsorBlock for YouTube
- Auto Tab Discard
- Clarify AI: YouTube Summaries
- Dark Reader
- Kagi Search for Firefox
- TWP - Translate Web Pages
- Bitwarden Password Manager

### Orion state

Orion app exists at:

- `/Applications/Orion.app`

Orion profile root:

- `/Users/conscience/Library/Application Support/Orion/Defaults`

Orion is currently running.

Important Orion files found:

- `favourites.plist`: existing favorites, small, 6 entries
- `reading_list.plist`: empty
- `browser_state.plist`: current windows/tabs
- `Extensions/extensions.plist`: extension registry exists but effectively empty
- `custom_filters.txt`: contains `reddit.com##.promotedlink`
- `history`: SQLite DB with `history_items`, `visits`, etc.

Orion current tabs:

- `https://help.kagi.com/orion/getting-started/importing.html#import_macos`
- `https://help.kagi.com/orion/index.html`
- `https://kagi.com/assistant/...` active tab

Current Orion pinned tabs: 0.

## Take

Do not copy raw Zen profile files into Orion. Zen is Firefox/Gecko based; Orion is WebKit/Safari-like with its own plist/SQLite state. Raw profile transplant is likely to corrupt or be ignored.

Best path:

1. Use standard import/export for bookmarks/history where Orion supports it.
2. Use generated HTML bookmark files for Zen spaces/pinned/current tabs.
3. Reinstall extensions manually in Orion and test compatibility one by one.
4. Recreate visual layout with computer-use screenshots once the tool is enabled.

## Step-by-step migration plan

### Phase 0: Enable visual automation

1. Enable computer-use for the platform you will use:
   - CLI: add `computer_use` to `platform_toolsets.cli`
   - Telegram: add `computer_use` to `platform_toolsets.telegram`
2. Prefer command: `hermes tools enable computer_use`.
3. If Hermes installs `cua-driver`, grant macOS permissions:
   - System Settings → Privacy & Security → Accessibility
   - System Settings → Privacy & Security → Screen Recording
4. Restart Hermes CLI/gateway.
5. Verify in a fresh session:
   - `hermes tools list` shows `✓ enabled computer_use`
   - a new session has the `computer_use` tool available
6. First tool action should be capture-only:
   - capture Zen app
   - capture Orion app

Success criterion: screenshots/AX trees are returned without raising windows or stealing focus.

### Phase 1: Backup before touching Orion

Before modifying Orion, quit Orion cleanly and back up its current profile:

- `/Users/conscience/Library/Application Support/Orion`

Also take a Zen backup after quitting Zen cleanly:

- `/Users/conscience/Library/Application Support/zen`

Reason: Zen is currently running, and direct SQLite/session reads are best-effort while the browser is live.

Suggested backup shape:

- `~/Workspace/browser-migration-backups/YYYY-MM-DD_HHMMSS/zen/`
- `~/Workspace/browser-migration-backups/YYYY-MM-DD_HHMMSS/Orion/`

Do not proceed with write/import steps until both backups exist.

### Phase 2: Export structured migration artifacts from Zen

Create generated artifacts under a working directory, for example:

- `~/Workspace/browser-migration/zen-to-orion/`

Artifacts to generate:

1. `zen-bookmarks.html`
   - Use Zen's built-in bookmark export if possible.
   - Fallback: export from `places.sqlite` after Zen is closed.

2. `zen-current-tabs-by-space.html`
   - Decode `zen-sessions.jsonlz4`.
   - Create folders:
     - `Zen pinned - lab`
     - `Zen essential - lab`
     - `Zen open tabs - lab`
     - `Zen pinned - joy`
     - `Zen open tabs - joy`
     - `Zen essential - no workspace`
   - Include title and URL.
   - Avoid including transient checkout/payment URLs.

3. `zen-extensions.md`
   - List extension name, Firefox extension id, active state, Orion install source candidate, and compatibility status.

4. `zen-containers.md`
   - Map Firefox containers to Orion equivalents.
   - Proposed mapping:
     - Work container → Orion profile/window named `Lab` or default profile with dedicated tab group
     - Personal container → Orion profile/window named `Joy`
     - Banking/Shopping → do not migrate cookies; use fresh logins or private/profile isolation

5. `zen-visual-reference/`
   - Screenshots from computer-use after enabled:
     - Zen full window
     - Zen sidebar/workspace switcher
     - Zen pinned/essential tab area
     - Zen settings relevant to appearance
     - Orion default layout before changes

### Phase 3: Import bookmarks into Orion

Preferred path:

1. Orion → File → Import From → Bookmarks HTML, if available.
2. Import `zen-bookmarks.html`.
3. Import `zen-current-tabs-by-space.html` as separate bookmark folders.
4. Keep current tabs as bookmarks first, not live tabs. Opening 100+ tabs in Orion immediately is a bad idea.

Why bookmarks first:

- Zen has 113 active tab records and 527 unique session URLs.
- Orion currently has only 3 tabs and no pinned tabs.
- Bulk-opening all Zen tabs would create noise, memory pressure, and possible login/security prompts.

Success criterion:

- Orion bookmarks contain the Zen bookmark hierarchy.
- Separate migration folders preserve pinned/essential/open tabs by Zen space.
- No existing Orion favorites are lost.

### Phase 4: Recreate pinned/essential tabs deliberately

Use the generated tab artifact, not the raw session file.

Recommended initial live Orion tabs:

- Start with Zen essentials only:
  - 6 no-workspace essentials
  - 6 `lab` essentials
- Then add pinned tabs from `lab` and `joy` if still useful.
- Leave non-pinned open tabs as bookmarks.

Reason: Zen has too much session state to blindly recreate. Orion should start clean.

Success criterion:

- Orion has a small intentional pinned set.
- Full old Zen session remains available in bookmarks for recovery.

### Phase 5: Extensions

Install and validate one by one in Orion.

Priority order:

1. Bitwarden Password Manager
   - Critical for logins.
   - Validate unlock/autofill manually.

2. uBlock Origin or Orion native content blocker equivalent
   - Orion has native content blocking plus custom filters.
   - Keep `reddit.com##.promotedlink` custom filter.
   - Test YouTube, Reddit, and common sites.

3. Kagi Search
   - Configure Kagi as default search provider if the extension is unnecessary or incompatible.

4. Dark Reader
   - Install only if Orion dark/theme settings are insufficient.

5. SponsorBlock
   - Test on YouTube.

6. TWP - Translate Web Pages
   - Test compatibility. Orion/WebKit may make translation extensions less reliable.

7. Auto Tab Discard
   - Maybe skip initially. Orion/WebKit memory behavior differs from Firefox; measure before adding.

8. Clarify AI YouTube summaries
   - Lowest priority. Test after core browsing works.

Success criterion:

- Each installed extension works on a known page.
- Broken extensions are disabled/removed immediately.
- Extension list and decisions are documented.

### Phase 6: Profiles / containers / privacy model

Do not try to migrate cookies/session storage from Zen containers.

Reason:

- Firefox containers are not the same as Orion profiles/tab groups.
- Copying cookies across engines is fragile and a security risk.
- Banking/shopping sessions should be clean logins.

Recommended mapping:

- Zen `lab` + Work container → Orion default profile or profile named `Lab`
- Zen `joy` + Personal container → Orion profile/window/tab group named `Joy`
- Banking and Shopping → clean, separate Orion profile if Orion profile isolation is good enough; otherwise keep them in Zen/Safari for now

Success criterion:

- Work and personal contexts are visually separable.
- Banking/shopping do not inherit cookies from old Zen.

### Phase 7: Visual layout recreation

Requires active computer-use.

Capture Zen visual state:

- Sidebar orientation and width
- Workspace switcher placement/icons
- Pinned/essential tab section
- Toolbar buttons and address bar placement
- Theme/dark mode/colors
- Favorites/bookmarks bar visibility

Then configure Orion manually with visual verification after each step:

1. Set sidebar/bookmarks/start page preferences.
2. Set default search to Kagi.
3. Set theme/dark mode.
4. Configure tab behavior and pinned tabs.
5. Create profiles/windows/tab groups if Orion supports the desired split.
6. Compare screenshots side by side.

Success criterion:

- Orion looks close enough for daily use.
- Differences from Zen are documented, especially anything Orion cannot emulate.

### Phase 8: Validation checklist

- Bookmarks imported and searchable in Orion.
- Pinned/essential tabs recreated intentionally.
- Kagi search works from address bar.
- Bitwarden unlock/autofill works.
- Ad blocking works on Reddit/YouTube/news sites.
- YouTube workflow works with SponsorBlock or accepted fallback.
- Dark mode is acceptable.
- Work/personal separation is acceptable.
- Zen remains untouched as rollback.
- Orion profile backup exists after migration.

## Risks and tradeoffs

- Orion may not fully support every Firefox extension.
- Firefox containers do not map cleanly to Orion.
- Zen-specific workspaces/essentials have no guaranteed native equivalent in Orion.
- Blindly opening 100+ old Zen tabs in Orion is likely counterproductive.
- Raw file-level copying between Zen and Orion is not recommended.
- Visual migration cannot be completed until computer-use is enabled and working in a fresh Hermes session.

## Open questions

1. Do you want Orion to become the daily driver immediately, or run in parallel with Zen for a week?
2. Should banking/shopping migrate to Orion, or stay isolated in another browser?
3. Should all historical Zen bookmarks move, or only current/pinned/workspace tabs plus recent bookmarks?
4. Do you want me to enable `computer_use` in Hermes config, or just leave the exact steps for you?

## Recommended next action

Enable `computer_use`, restart Hermes, then do a visual capture-only pass of Zen and Orion. After that, generate HTML migration artifacts and import into Orion in controlled phases.
