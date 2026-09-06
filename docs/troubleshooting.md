# Troubleshooting

- **`dotagents doctor`** is the first stop: it validates skill frontmatter, role definitions, lock pins, materialized copies, hook registration, and audits external sources.
- **Upgrading from an old config** that targets agent `pi` with MCP: vanilla Pi has no MCP surface, so the target is ignored with a warning — rename `pi` to `omp` in `dotagents.yaml` if you meant the OMP fork.
- **A sync proposed removals you didn't expect:** setup-driven syncs always preview removals per harness and default to keeping your files; answer `n` and inspect with `dotagents status`.
- **"exists but is not a symlink" conflict on a root instruction file:** a real file occupies the harness's memory path (e.g. `~/.claude/CLAUDE.md`). dotagents never overwrites it silently — migrate the content into `~/.agents/AGENTS.md`, delete the real file, and rerun `dotagents sync`.
- **Memory tools skipped during sync:** no Go toolchain on PATH. Install Go, or copy prebuilt binaries to `$GOBIN`/`~/.local/bin` manually; sync will manage them from then on (rebuilds only when sources change).
