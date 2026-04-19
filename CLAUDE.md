# CLAUDE.md

IMPORTANT: Follow these rules at all times.

## Critical Rules

- NEVER take shortcuts without asking — STOP, ASK, WAIT for approval.
- NEVER force push — make a new commit to fix mistakes.
- NEVER commit secrets to version control.
- Only do what was asked — no scope creep.
- Understand existing code before changing it.
- If you don't know, say so — never guess.
- Test before and after every change.
- Surface errors clearly — no silent fallbacks.

## Project Conventions

**Status:** Ported from markwharton/mcp-bridge-go, informed by patterns from markwharton/plankit (mature, production-tested). Conventions below describe the Go stack as implemented.

### Stack & Layout
- **Go**, stdlib only — zero external dependencies. Floor version declared in `go.mod`.
- Entry point: `cmd/mcp-bridge/main.go`. Internal packages under `internal/` (e.g., `internal/bridge`, `internal/config`, `internal/version`).
- Subcommand routing via `os.Args` + `flag.FlagSet` per command — no cobra/cli libraries.
- Version injected at build time via `-ldflags -X .../internal/version.version=...` (lowercase, per plankit).

### Build & Test
- **`CGO_ENABLED=0` by default** in the Makefile — pure-Go static binaries, no implicit glibc dep on linux.
- **Tests override to `CGO_ENABLED=1`** because `go test -race` requires cgo. Make this explicit in the `test` target.
- **Build:** `make build` (current platform → `dist/`), `make build-all` (darwin amd64/arm64, linux amd64/arm64, windows amd64).
- **Test:** `make test` (runs `CGO_ENABLED=1 go test -v -race ./...`). Use `httptest.NewServer` for HTTP tests and `t.TempDir()` for filesystem tests.
- **Lint / format:** `make lint` runs `go vet` + a `gofmt -l` drift check (fails if any tracked package needs formatting). `make fmt` (`go fmt`) is the autofix.
- **Install:** `make install` → `$GOPATH/bin`.

### Branches & Release
- **Development branch:** `develop`. All work happens here.
- **Never commit directly to `main`.** `pk guard` enforces this.
- **Release:** `make release` calls `pk release`, which merges `develop` → `main` and pushes. Tag-triggered CI builds cross-platform artifacts and publishes a GitHub release. No bespoke `scripts/release.sh`.

### CI/CD
- **Go version source of truth is `go.mod`.** In every workflow, use `actions/setup-go` with `go-version-file: 'go.mod'` — do not pin a version string in the workflow, and do not use `'stable'` (moving target).
- **Pin GitHub Actions to full commit SHAs** with a `# vX.Y.Z` comment for readability. Mutable tags (`@v4`) are a supply-chain risk.
- **`.github/dependabot.yml`** with a `github-actions` ecosystem entry keeps pinned SHAs updated automatically.
- **Release builds set `CGO_ENABLED: 0`** explicitly in the workflow build env (mirrors the Makefile default, prevents surprise).
- Workflows: `ci.yml` (build/test/lint + coverage on push/PR to `develop`/`main`), `release.yml` (tag-triggered, matrix builds, SHA-256 checksums, GitHub release).

### Config & Safety
- Prefer atomic writes for any file the tool mutates: temp file + rename, with `.bak` backup.
- Use injectable function variables (e.g., path resolvers) so tests can override without touching the real filesystem.
