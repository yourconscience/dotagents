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

## Rules

- Always include direct links. Never fabricate quotes or links.
- Run `gh api` calls in parallel where possible.
- Clone to ~/Public, not to the current working directory.
- Never install or execute code from evaluated repos.
- If `x-cli` auth is broken, fall back to WebSearch for X.com signal.
- For find mode, cap at 15 candidates for search, 5 for sentiment, 3 suggested for deep eval.
