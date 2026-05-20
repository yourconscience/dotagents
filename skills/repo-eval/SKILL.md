---
name: repo-eval
description: Find, triage, and deep-evaluate GitHub repos for a given need. Use when the user says /repo-eval, asks to find a tool or library, wants to vet a specific repo, or needs to compare alternatives before adopting one.
---

# repo-eval

Evaluate GitHub repos as candidates for adoption. Two modes determined by input shape.

## Usage

```
/repo-eval find <need>          # discovery + light triage
/repo-eval <owner/repo> [concern]  # deep eval of a known repo
```

## Find mode

Input is a need description, not a repo slug.

### 1. Search GitHub

```bash
gh search repos "<need>" --json fullName,description,stargazersCount,pushedAt,isArchived,license,url --limit 15
```

Filter out archived repos and forks. Sort by stars descending.

### 2. Light triage (all candidates in parallel)

For each candidate, fetch via `gh api`:

```bash
# Repo metadata
gh api repos/<owner>/<repo>

# Last commit date, latest release, SECURITY.md presence
gh api graphql -f query='query($owner:String!,$name:String!){
  repository(owner:$owner,name:$name){
    defaultBranchRef{ target{ ... on Commit { committedDate } } }
    latestRelease{ tagName publishedAt }
    securityRoot: object(expression:"HEAD:SECURITY.md"){ __typename }
  }
}' -F owner=<owner> -F name=<name>

# Top 7 open issues by reactions
gh api graphql -f query='query($q:String!){
  search(type:ISSUE,query:$q,first:7){
    nodes{ ... on Issue { title url reactions{totalCount} createdAt comments(last:5){
      nodes{ createdAt authorAssociation }
    }}}
  }
}' -F q="repo:<owner>/<repo> is:issue is:open sort:reactions-desc"
```

Assess each repo on:
- **Activity**: last commit within 30/60/180 days, recent release
- **Maintenance**: maintainer replied to top issues within 60 days
- **Friction**: top issues mentioning crash, data loss, broken, abandoned, install problems
- **Basics**: license present, not archived, security policy exists

### 3. Community sentiment (top 3-5 candidates)

For candidates that pass light triage, check community signal scoped to the repo/tool name:

- **HN**: `https://hn.algolia.com/api/v1/search?query=<repo-name>&tags=story&hitsPerPage=5`
- **X.com**: `x-cli search "<repo-name>" --type top --count 5` (fallback: `site:x.com` via WebSearch)
- **Reddit**: search relevant subreddits or `site:reddit.com <repo-name>` via WebSearch

### 4. Output

Present a ranked shortlist:

```
## <Need> - Repo Shortlist

| Repo | Stars | Last commit | Release | Top issue friction | Sentiment |
|------|-------|-------------|---------|-------------------|-----------|

### Recommendation
Which repos to deep-eval and why. Proactively suggest running eval mode
on the top 2-3 via AskUserQuestion.
```

After the user picks candidates, run eval mode on each in parallel via subagents.

## Eval mode

Input is an `owner/repo` slug, optionally followed by a specific concern.

### 1. Full GitHub analysis

Same `gh api` calls as light triage, but also:
- Read top 7 issues in full (title, body snippet, comment count, maintainer response)
- Check dependency count and language breakdown: `gh api repos/<owner>/<repo>/languages`
- Check recent commit frequency: `gh api repos/<owner>/<repo>/stats/commit_activity`

### 2. Community sentiment

Same as find mode step 3, but deeper: read the top 2-3 HN threads and Reddit threads in full, not just titles.

### 3. Clone and code analysis

```bash
git clone --depth 1 https://github.com/<owner>/<repo>.git ~/Public/<repo>
```

Analyze:
- README quality and completeness
- Dependency manifest (package.json, go.mod, requirements.txt, Cargo.toml)
- Look for red flags: vendored secrets, excessive permissions, suspicious install scripts
- If user provided a specific concern, search the codebase for it

For ML/model repos, also check model-distribution surfaces without executing anything:
- Hugging Face model/dataset API if the README links weights or datasets: `https://huggingface.co/api/models/<org>/<model>` or dataset equivalent. Capture public/private state, downloads/likes, tags, lastModified, and file list.
- Artifact safety: flag pickle / arbitrary-code-loading formats as trusted-code artifacts; do not load them during eval.
- Setup scripts: read shell/bootstrap scripts for side effects such as `sudo`, package-manager changes, modprobe/kernel tweaks, telemetry/API-key prompts, or accelerator-specific installs. Report these as operational concerns, not necessarily malicious findings.
- Versioning: note missing releases/tags and recommend pinning by commit SHA when no release exists.

Do NOT install or run the tool. Analysis is read-only.

### 4. Output

```
## <owner/repo> - Deep Eval

### Health
Stars, commits, releases, maintainer responsiveness, license, security policy.

### Top Issues
List top 5 with title, reactions, age, and whether maintainer responded.

### Community Sentiment
What HN/X/Reddit say. Quote notable opinions with links.

### Code Analysis
README quality, dependencies, red flags, concern-specific findings.

### Verdict
Worth adopting / Proceed with caution / Avoid. One paragraph explaining why.
```

If the user asks for a file/report, write Markdown under `~/Workspace/reports/` with a descriptive date-stamped filename, then verify the file exists and report its absolute path. Include a concise executive verdict near the top, source list, direct links, and explicit caveats for skipped sources.

### 5. Offer setup

After presenting the verdict and the user indicates which repo they want, **ask whether they want you to set it up**. The evaluation was read-only; now the user needs the tool running. Do not wait for them to ask — they often assume setup follows evaluation.

## Rules

- Always include direct links. Never fabricate quotes or links.
- If the user asks to evaluate an announcement/post/article that contains a GitHub link, keep the announcement's claims and discussion as the primary object. Use `/repo-eval` on the linked repo as supporting evidence only. Do not reframe the whole task as repo adoption unless the user explicitly asks to adopt/vet that repo.
- Run `gh api` calls in parallel where possible.
- Clone to ~/Public, not to the current working directory.
- **During evaluation (steps 1-4):** Never install or execute code from evaluated repos. Analysis is read-only.
- **After the user picks a winner (step 5):** Offer to install, configure, and launch the recommended tool. At that point, following the repo's own setup instructions is expected and safe.
- If `x-cli` auth is broken, fall back to WebSearch for X.com signal.
- For find mode, cap at 15 candidates for search, 5 for sentiment, 3 suggested for deep eval.
