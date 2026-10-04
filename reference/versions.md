# reference/versions.md — Single source of truth for pinned versions

This file is the **authoritative** record of every pinned version used in this
repository (SPEC §11.3, C05/N33, C01GO). It is referenced by `SPEC.md`,
`AGENTS.md`, `go.mod`, the Makefile, and the CI workflow. **Staleness is a
defect** — when a newer stable is available, upgrade; never suppress.

---

## Go & runtime dependencies

Pinned **latest-stable** versions (SPEC §11.3). These must match `go.mod`
exactly for the build and CI to be consistent.

| Component | Version | Reference |
|-----------|---------|-----------|
| Go | **1.27.1** (`go 1.27.1` in `go.mod`; CI `GO_VERSION: "1.27"`) | C01GO/N01GO |
| MCP Go SDK | **v1.8.0** (`github.com/modelcontextprotocol/go-sdk`) | M01 |
| resty.dev/v3 | **v3.0.0-rc.4** (L02GO; no stable yet — latest available) | L02GO |
| prometheus client_golang | **v1.24.1** (O01–O04) | O01–O04 |
| logrus | **v1.10.2** (L01GO) | L01GO |
| envconfig | **v1.4.0** (env-driven config) | env-driven config |

> Note: `resty.dev/v3` is still a release candidate (`v3.0.0-rc.4`); it is the
> latest available and is pinned deliberately (SPEC §11.3).

---

## Local / CI toolchain

Versions detected on the local development machine (darwin/arm64) at the time
Phase 1 scaffolding was authored. CI installs these at the same versions so
local and CI behaviour match.

| Tool | Version | Purpose |
|------|---------|---------|
| golangci-lint | **2.14.0** | `make lint` (gofmt/gofumpt/govet/gosec/staticcheck/errcheck/ineffassign/unused/govulncheck) |
| go-arch-lint | **1.19.0** | layer-edge enforcement (`.go-arch-lint.yml`, C07GO) |
| gremlins | **0.6.0** | mutation testing hard gate (C02/C08GO) |
| gitleaks | **8.30.1** | secret scan over git history (C03/N28) |
| goreleaser | **2.18.2** | `make build` single-binary-per-platform artifacts (B01/B02/B04) |
| gosec | **2.29.0** | static security analysis (`make lint`) |
| govulncheck | **v1.8.0** | vulnerability scan (`make lint`) |
| docker | **29.8.1** | `make container-image` build/push |

> When upgrading any tool, update it here **first**, then align the Makefile /
> CI install steps, then run `make lint` / `make test` / `make build` to confirm.
