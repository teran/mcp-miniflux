# AGENTS.md

Guidance for AI agents working in this repository. `AGENTS.md`, like `SPEC.md`,
is **strictly English** (D02). The README is English by default (D01). When in
doubt, the authoritative source is [`SPEC.md`](SPEC.md) — this file is a concise
operating manual and must never contradict it.

## This repository

- **`mcp-miniflux`** is a **stateless Remote (HTTP) MCP server** that exposes the
  Miniflux RSS reader API (`https://miniflux.app`) as a set of specific,
  strongly-typed MCP tools. It runs as a **sidecar container next to the
  Miniflux instance**; Miniflux owns **all** state (feeds, categories, entries,
  counters, OPML). The server holds no persistent state of its own.
- **Transport & auth:** clients connect over **Streamable HTTP** on `:8080`
  (`LISTEN_ADDR`). The server **passes the inbound `X-Auth-Token` straight
  through** to Miniflux as `X-Auth-Token`; if absent it falls back to the
  `MINIFLUX_API_TOKEN` env var. **No OAuth2, no in-server TLS** (reverse proxy's
  job). Observability lives on a separate `:8081` (`INTERNAL_ADDR`).
- **Implementation language:** **Go 1.27.1** (see `go.mod` / CI `GO_VERSION`).
- **Layout:** layered DDD/Clean with **top-level named packages** — `internal/`
  is intentionally **NOT used** (no visibility restriction justified, recorded
  in SPEC §6.1). Top-level packages: `cmd/mcp-miniflux/main.go` (thin composition
  root) + `domain/` (pure types & ports) + `application/` (use-case handlers,
  tool registry) + `infrastructure/` (config, logging, mcp, miniflux client,
  observability). Layer edges are enforced by **go-arch-lint** (`.go-arch-lint.yml`):
  `cmd → application/infrastructure`, `application → domain`,
  `infrastructure → domain`, `domain → (nothing)`. Any other edge fails CI.
- Pinned versions (single source of truth is `reference/versions.md`): Go 1.27.1,
  MCP go-sdk v1.8.0, resty.dev/v3 v3.0.0-rc.4, logrus v1.10.2, prometheus
  client_golang v1.24.1, envconfig v1.4.0. **Staleness is a defect** — upgrade,
  never suppress.

## Build system (R07) & quality gates (CI enforces these)

CI binds to the **build-system interface**: it calls `make <target>`, not raw
`go ...` commands. Never bypass the Makefile in CI or in your workflow.

- `make build` — **goreleaser** → single binary per platform into `dist/`.
- `make test` — `go test -race` + **coverage ≥ 95% gate**; **fails below 95%**.
  Never ship a change that lowers coverage; add tests with the code.
- `make lint` — golangci-lint (incl. gofmt/gofumpt), `go vet`, gosec,
  govulncheck, go-arch-lint. **Findings are FIXED, never suppressed** (no blanket
  `#nosec`, no default excludes).
- `make e2e` — e2e via **go-docker-testsuite** (`go test -tags e2e ./...`,
  `//go:build e2e` build-tagged, excluded from the default unit pass). Runs in a
  dedicated CI job.
- `make container-image [push=true]` — build-only / build+push the image **from
  the `build` artifact** (distroless `nonroot`, no in-image compilation).

**Dedicated hard gates** (not part of the make interface — separate CI jobs):

- **Mutation testing (gremlins)** — `gremlins unleash ./... --threshold-efficacy=80
  --threshold-mcover=80` is a **hard gate that fails the build** below 80%.
  Weak/surviving-mutant tests are a defect; improve the tests, never lower the
  threshold or exclude packages.
- **Secret scan (gitleaks)** — `gitleaks detect --source . --redact --verbose`
  over **git history**; findings **fixed**, never suppressed.

## TDD workflow

Tests are written **first** by **@qa** (isolated context); the implementation is
written by **@developer** (isolated context). Do **not** weaken tests to make the
implementation pass. e2e verifies the full handler→upstream path including
pass-through `X-Auth-Token` and live redaction of `secret:true` fields.

## Working in this repo

- **Statelessness & no implicit startup actions (A02/N35):** startup does **local
  wiring only** — load config, build logger, emit the startup banner (first log
  line), compose layers, start listeners. **NEVER** make an outbound/upstream
  request at startup (no connectivity/health pings to Miniflux, no eager fetches,
  no auth-validation calls). Every upstream request happens **only inside a tool
  call**; each tool call maps to one upstream request. Liveness probes reflect
  process liveness, **not** upstream reachability.
- **Security-first:** TLS is never in-server. Model every Miniflux request,
  response and config field as a **struct**; annotate secret-bearing fields
  **`secret:true`** (feed `password`, `MINIFLUX_API_TOKEN`). A **single redaction
  helper** strips annotated fields from tool output and logs — no regex guessing,
  the struct is authoritative. **Never log secrets/tokens** (L05).
- **Tool contracts (S08/S09):** every `tools/call` validates arguments against its
  `inputSchema` (`additionalProperties:false`) **before** execution (invalid →
  `InvalidParams`); output conforms to `outputSchema` and is passed through an
  ANSI/control-sequence sanitizer. Tools are grouped read → write/update → delete
  (S03). `idempotentHint:true` only where a real data-level mechanism exists
  (search-before-create, state-setting, Miniflux 200-on-duplicate); toggling tools
  are deliberately NOT idempotent. `destructiveHint:true` tools require human
  confirmation (HITL, S12). The single `openWorldHint:true` tool
  (`discover_subscriptions`) returns **untrusted data structurally**
  (`structuredContent` + `outputSchema`, typed candidates), never raw free-form
  text, and is isolated from secrets/filesystem.
- **Outbound HTTP:** use **resty.dev/v3** (never bare `http.Client`/`http.Get`).
  Set an explicit timeout/context; **do NOT add retry/backoff** — on failure
  (including `429`) return the error to the model and log it (no secrets). No
  shell/SQL/URL string concatenation (S11).
- **Logging & observability:** logrus. HTTP mode (`-mode http`, the deployed
  mode): logs to **stdout**, always enabled at default `info`. `-mode stdio`
  (debug only): logs to a **file** (`LOG_FILENAME`, chmod 600), enabled only when
  `LOG_LEVEL` is set. Default format text; `LOG_FORMAT=json` for JSON. Wire the
  SDK `slog` logger into logrus (`ServerOptions.Logger`). Log a **per-request
  tool-call access log at `info`** (no `debug` gate) with structured fields:
  `tool`, **redacted** `args`, `source`, `duration`, `outcome`. **`request_id`**
  correlation: generate per request, thread through context on every log record,
  and propagate to Miniflux as `X-Request-ID`. Observability on `:8081`
  (`INTERNAL_ADDR`) serves Prometheus metrics + pprof + probes; **always present**
  for this Remote server (no opt-out).
- **Pass-through design:** inbound `X-Auth-Token` is forwarded to Miniflux as
  `X-Auth-Token`. Token resolution order: (1) inbound `X-Auth-Token` header, (2)
  `MINIFLUX_API_TOKEN` env. The resolved token is used verbatim and never logged.
- **Docs:** keep `SPEC.md` and `AGENTS.md` in English. The README stays English
  and begins with the AI-Generated Content disclaimer (D03). This disclaimer is
  **README-only** — do not add it to AGENTS.md.

## Definition of done

A change is done only when **all** of the following hold:

- Tests were written first (TDD); unit tests pass with **coverage ≥ 95%**
  (`make test`).
- **Lint clean** (`make lint`): golangci-lint, `go vet`, gosec, govulncheck,
  go-arch-lint — **no findings left unfixed, none suppressed**.
- **gremlins mutation gate** ≥ 80/80 passes (dedicated CI job).
- **gitleaks** secret scan over git history passes; no secrets committed.
- When e2e exist: the **e2e job** (`make e2e`, go-docker-testsuite) passes.
- Docs updated as needed; `SPEC.md`/`AGENTS.md` still consistent and in English.
- **No secrets leaked** into outputs or logs; redaction helper covers new
  `secret:true` fields.
- **Statelessness respected**: no startup outbound/upstream I/O, no implicit
  actions, no retry/backoff, tool calls map 1:1 to upstream requests.

**Repository conventions:** default branch `master` (never `main`); module path
`github.com/teran/mcp-miniflux`; CI is GitHub Actions with stages linters →
build → tests → release artifacts, all bound to `make <target>`.
