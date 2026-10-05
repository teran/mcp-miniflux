# SPEC.md — `mcp-miniflux`

**A Go MCP server acting as a proxy/adaptor to the Miniflux RSS reader API**
(`https://miniflux.app`).

This document is the authoritative specification for building, securing,
observing and operating the server. It follows the recommended SPEC outline of
the `mcp-server-spec` skill. Every section carries the criterion codes it
satisfies (see `reference/criteria/common.md`, `reference/criteria/go.md` and
`reference/spec-conformance.md`), so a reviewer can trace each `MUST` to a
section. Codes are permanent identifiers.

---

## 1. Overview

`mcp-miniflux` is a **stateless Remote (HTTP)** MCP server that exposes the
Miniflux RSS reader API as a set of specific, strongly-typed MCP tools. It is
deployed as a **sidecar container next to the Miniflux instance** and lets an
MCP-capable model-driven client (an LLM agent) read, write and manage the user's
feed subscriptions, categories and entries through a safe, least-privilege,
secrets-safe tool surface.

Who calls it:

- **MCP clients** (LLM agents / assistants) connect over **Streamable HTTP** to
  the MCP listener on `:8080`.
- The server, in turn, calls the **Miniflux HTTP API** (`https://miniflux.app`,
  base path `/v1/...`) on behalf of each tool call, **passing the inbound
  `X-Auth-Token` straight through** to Miniflux.

External system: Miniflux (`https://miniflux.app`) — a self-hostable RSS/Atom
reader. It owns **all** state (feeds, categories, entries, counters, OPML). The
server holds no persistent state of its own (see §8, X01).

Language and profile: **Go** — the default/reference profile of the skill
(C02GO). Pinned versions are recorded in §11 (C05/C01GO) and come from the
single source of truth `reference/versions.md`.

---

## 2. Repository & CI

- **Repository host & path (R05, decided before development):** public GitHub,
  `github.com/teran/mcp-miniflux`. Standard **public** module path
  `github.com/teran/mcp-miniflux` (S06 — public GitHub repos use standard
  public paths; the internal/Forgejo local-only rule does not apply).
- **Default branch (R02):** `master` (never `main`).
- **CI provider (R06, chosen explicitly):** **GitHub Actions** — the native CI
  of the repository host (GitHub repo → GitHub Actions). Single provider, as
  permitted when explicitly specified.
- **Language profile:** Go profile (`reference/criteria/go.md`), default
  (C02GO).
- The 4-stage pipeline shape (linters → build → tests → release artifacts) is
  provider-agnostic and is bound to the standard **build-system interface**
  (R07) — `make lint` / `make test` / `make build` /
  `make container-image [push=true]` (see §11).

---

## 3. Transports & Auth decisions

### M02 — Transport: HTTP — Streamable HTTP

**Decision:** The primary transport is **HTTP — Streamable HTTP**, using the
official Go SDK's support for it.

**Justification (why not STDIO):**
- This is a **Remote** server (M06) deployed as a network sidecar.
- It serves **multiple concurrent MCP clients** (multiple LLM agents / one or
  more gateways) — STDIO is a single-process, single-client, local-only
  transport and does not fit a shared sidecar.
- It is a **long-running** server living next to the Miniflux instance — HTTP
  is the natural fit, and Streamable HTTP (the current preferred flavour, as
  opposed to legacy HTTP+SSE) is directly supported by the pinned official
  go-sdk (v1.8.0).
- A **reverse proxy** (TLS terminator, see §7/S01) fronts the sidecar and
  forwards the MCP transport to the `:8080` listener; observability lives on a
  separate `:8081` (see §10).

**STDIO** remains available **optionally** as a `-mode stdio` debugging/local
path (a single client on the same host). It is **not** the deployed mode.

### M03 — Auth: NO OAuth2 (pass-through API token)

**Decision:** **No OAuth2** is used for either direction.

**Justification (grounded in the upstream API):**
- Miniflux authenticates its API via its own **per-application API key**, sent
  in the `X-Auth-Token` HTTP header. There is **no OAuth2-for-API** in Miniflux.
  Per M03, OAuth2 is meaningful **only if the upstream API supports OAuth2
  usable with the API**; it does not, so we skip it and use the native token.
- **Inbound** (MCP client → server): the MCP server authenticates inbound HTTP
  requests by requiring the same `X-Auth-Token` header.
- **Outbound** (server → Miniflux): the server **passes the API token straight
  through** as `X-Auth-Token` to the Miniflux API — the requested
  "pass-through" design. There is no token storage, rotation or conversion.

**Configuration (env-driven):**
- `MINIFLUX_API_URL` — base URL of the Miniflux instance (e.g.
  `https://reader.example.com`).
- `MINIFLUX_API_TOKEN` — the Miniflux API token used by the server's own
  outbound calls and/or the default when a client does not supply one (see
  §3.1 token resolution). Marked `secret:true` (S02, §7).

### M06 — Deployment type & launch mode

- **Deployment type:** **Remote (HTTP)** — deployed as a **sidecar container
  next to the Miniflux instance**.
- **Launch mode parameter:** `-mode http|stdio`, **default `stdio`** (per M06).
  Because this is a Remote server whose **primary/effective mode is `http`**,
  the deployed container is **always started with `-mode http`** (set in the
  container `ENTRYPOINT`). `-mode stdio` is only for local debugging.
- **Derived consequences:**
  - Container image **MUST** be built and published via CI/CD (R01/B03) — see
    §11.
  - Logging channel **stdout** (12-factor) (L01); logging **always enabled at
    default `info`** in HTTP mode (L02) — see §9.
  - Internal observability endpoint **always present** on `:8081`
    (O01/O04/N32) — see §10.
  - `LISTEN_ADDR` default `:8080`, `INTERNAL_ADDR` default `:8081` (O04).

### 3.1 Token resolution for outbound Miniflux calls

The outbound `X-Auth-Token` for a tool call is resolved in this order:

1. The inbound request's own `X-Auth-Token` header, **if present** (true
   pass-through: the client authenticates itself to Miniflux directly).
2. Otherwise, `MINIFLUX_API_TOKEN` from the environment (server-side token for
   clients that authenticate only to the MCP server).

The resolved token is used verbatim and is never logged (L05, §9). No secret
ever appears in tool output or logs (S02, §7).

---

## 4. Tool surface

**Design rules applied (S03, M04, M05, M07, S08, S09, X02):**

- **Specific, explicitly named, strongly typed tools** (M07/N29) — no generic
  CRUD/passthrough mega-tool. Each tool has its own `inputSchema`
  (`additionalProperties:false`, strict types) and `outputSchema`.
- **Complete use cases** (M05/N10): each tool is one finished task end-to-end;
  no client chaining of low-level calls is required to accomplish a task.
- **Grouped read → write/update → delete** (S03).
- **Least privilege (S10):** user management and API-key management are
  **excluded** from the tool surface (see §7). Only a `get_me`/current-user
  **read** is exposed for user context.
- **Secrets-safe (S02):** every Miniflux request/response is modeled as a
  struct; fields that may hold secrets (feed `password`, and the API token)
  are annotated `secret:true`; a single redaction helper strips them from tool
  output and logs (§7).
- **Idempotency (X02/N25):** `idempotentHint: true` is set **only** where a
  real data-level mechanism exists (search-before-create, state-setting,
  Miniflux 200-on-duplicate). Toggling tools are deliberately NOT idempotent.
- **HITL (S12):** every `destructiveHint: true` tool is human-confirmable and
  deniable; the server never cascades destructive calls automatically.

**Input-validation contract (S08):** all `tools/call` arguments are validated
against the tool's `inputSchema` (`additionalProperties:false`) **before** any
upstream request; invalid arguments are rejected with MCP `InvalidParams`.

**Output contract (S09):** tool output conforms to the tool's `outputSchema`;
all string text is passed through a sanitizer that strips ANSI/control escape
sequences before returning text to the client.

### 4.1 Read tools (`readOnlyHint: true`, `openWorldHint: false`)

> All read tools map 1:1 to a single upstream Miniflux GET. They never modify
> state. Where a redaction contract is noted, the redaction helper (S02) strips
> `secret:true` fields (e.g. feed `password`) from the output.

**`list_feeds`** — list all feed subscriptions, optionally filtered by category.
- Inputs: `category_id` (`int`, optional), `limit`/`offset` (`int`, optional).
- Outputs: `feeds[]` (id, user_id, feed_url, site_url, title, category,
  status, error_count, …; `password` redacted).
- Annotations: `title` "List feeds", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns the user's feed subscriptions. Use to enumerate feeds
  or to look up a feed id before calling `get_feed`/`update_feed`/`delete_feed`.
  Filter by `category_id` to scope to one category. Read-only."
- Upstream: `GET /v1/feeds`.

**`get_feed`** — return a single feed by id.
- Inputs: `feed_id` (`int`, required).
- Outputs: full feed object (`password` redacted).
- Annotations: `title` "Get feed", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns one feed by id. Use the id from `list_feeds`. Read-only."
- Upstream: `GET /v1/feeds/{feedID}`.

**`list_categories`** — list categories with per-category entry counts.
- Inputs: none.
- Outputs: `categories[]` (id, title, feed_count, entry_count).
- Annotations: `title` "List categories", `readOnlyHint:true`,
  `openWorldHint:false`.
- Instructions: "Lists categories with counts. Use category ids with
  `list_category_feeds`/`mark_category_entries_read`/`delete_category`.
  Read-only."
- Upstream: `GET /v1/categories?counts=true`.

**`list_entries`** — list entries across the whole account with filters.
- Inputs (all optional): `status` (`unread|read|removed`), `order`
  (`id|status|published_at|category_title`, default `published_at`),
  `direction` (`asc|desc`, default `desc`), `limit` (`int`, default 100,
  max 1000), `offset` (`int`), `search` (`string`), `starred`
  (`bool`), `category_id` (`int`), `before`/`after` (RFC 3339 timestamps).
- Outputs: `total`, `entries[]` (id, user_id, feed_id, status, starred, title,
  url, comments_url, published_at, created_at, content, …).
- Annotations: `title` "List entries", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns entries with the given filters. This is the main
  'what is in my queue' tool. Use `status` to target unread/read/removed,
  `search` for full-text, `starred` for bookmarks, `category_id` to scope, and
  `before`/`after` for time windows. Read-only."
- Upstream: `GET /v1/entries`.

**`get_entry`** — return a single entry by id.
- Inputs: `entry_id` (`int`, required).
- Outputs: full entry object.
- Annotations: `title` "Get entry", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns one entry by id, including its full content. Use the id
  from `list_entries`/`get_feed_entries`. Read-only."
- Upstream: `GET /v1/entries/{entryID}`.

**`get_feed_entries`** — list entries of a single feed with filters.
- Inputs: `feed_id` (`int`, required); plus the `status`/`order`/`direction`/
  `limit`/`offset`/`search`/`starred`/`before`/`after` filters from
  `list_entries`.
- Outputs: `total`, `entries[]`.
- Annotations: `title` "Get feed entries", `readOnlyHint:true`,
  `openWorldHint:false`.
- Instructions: "Lists entries for one feed. Use to read the contents of a
  specific subscription. Read-only."
- Upstream: `GET /v1/feeds/{feedID}/entries`.

**`get_counters`** — feed-level unread counts.
- Inputs: none.
- Outputs: `feeds` (feed_id → unread count) plus `totals` (unread/read counts).
- Annotations: `title` "Get counters", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns per-feed and total unread/read counters. Useful for
  summarizing what needs attention. Read-only."
- Upstream: `GET /v1/feeds/counters`.

**`get_me`** — current user profile.
- Inputs: none.
- Outputs: `me` object (id, username, is_admin, theme, …). No secrets exposed.
- Annotations: `title` "Get current user", `readOnlyHint:true`,
  `openWorldHint:false`.
- Instructions: "Returns the authenticated user's profile. The only user-related
  tool; user *management* is intentionally out of scope (least privilege).
  Read-only."
- Upstream: `GET /v1/me`.

**`export_opml`** — export the whole subscription set as OPML.
- Inputs: none.
- Outputs: `opml` (`string`, the OPML XML document).
- Annotations: `title` "Export OPML", `readOnlyHint:true`, `openWorldHint:false`.
- Instructions: "Returns the user's full subscription set as OPML XML. Use for
  backup/migration. Read-only. The returned document is trusted local data; it
  is returned structurally as the `opml` field."
- Upstream: `GET /v1/export`.

**`discover_subscriptions`** — probe a URL and return candidate feeds.
- Inputs: `url` (`string`, required — the URL to probe; strict format, must be
  an absolute `http(s)` URL, see §7).
- Outputs: `feeds[]` (candidates: `url`, `title`, `type`).
- Annotations: `title` "Discover subscriptions", `readOnlyHint:true`,
  **`openWorldHint:true`** (see justification below).
- Instructions: "Takes an arbitrary URL and asks Miniflux to detect the feed(s)
  it exposes. **This is an open-world tool**: the URL and the returned
  candidates are untrusted external data. It is isolated — it cannot read any
  local data and returns only the candidate list, structurally, never raw
  free-form text. Use before `create_feed` to confirm a feed URL. Read-only
  with respect to the account."
- Upstream: `POST /v1/discover` (body `{ "url": "<url>" }`).

> **Open-world decision for `discover_subscriptions` (kept):**
> We **keep** this tool because feed discovery is a core, high-value Miniflux
> use case (find the feed URL before subscribing) and Miniflux natively exposes
> it. Because it takes an arbitrary external URL to probe, it is declared
> `openWorldHint: true` and treated as an **open-world** tool under S07/S10:
> its output is **untrusted data returned structurally**
> (`structuredContent` + `outputSchema`, candidates as typed objects), never
> raw free-form text (N20). It is **isolated** (S10): no `ALLOW_DIRS`, no
> secrets, no elevated scope, and its input is strictly validated (S08) — the
> URL must be an absolute `http`/`https` URL and is passed to Miniflux only
> (SSRF exposure is bounded to the trusted Miniflux instance; §7/S11). This is
> the single `openWorldHint: true` tool in the surface.

### 4.2 Write / Update tools (`readOnlyHint: false`, `destructiveHint: false`)

> Write/update tools map to upstream `POST`/`PUT` calls. `idempotentHint` is
> set only where a real data-level mechanism exists (X02/N25).

**`create_feed`** — subscribe to a feed.
- Inputs: `feed_url` (`string`, required), `category_id` (`int`, optional),
  `title` (`string`, optional), `username`/`password` (`string`, optional —
  `password` is `secret:true`, S02).
- Outputs: the created feed object (`password` redacted).
- Annotations: `title` "Create feed", `readOnlyHint:false`,
  `idempotentHint:true` (**search-before-create**, see below).
- Instructions: "Subscribes to the feed at `feed_url`. **Idempotent**: the
  server first searches existing feeds by URL; if the feed already exists it
  returns the existing feed instead of creating a duplicate. Safe to retry. Use
  `discover_subscriptions` first to confirm the URL. Optional HTTP credentials
  are passed to Miniflux and never returned/logged."
- Upstream: `POST /v1/feeds`.
- **Idempotency mechanism (X02):** the handler performs a **search-before-create**
  — it matches an existing feed by `feed_url`/`site_url` (via `list_feeds`) and
  returns it without POSTing when found. This is a genuine data-level
  idempotency mechanism, so `idempotentHint:true` is valid.

**`update_feed`** — update a feed's metadata/credentials.
- Inputs: `feed_id` (`int`, required); `title`, `category_id`, `site_url`,
  `username`/`password` (`secret:true`), `user_agent`, `scraper_rules`,
  `rewrite_rules`, `crawler` (`bool`), etc. (all optional).
- Outputs: the updated feed object (`password` redacted).
- Annotations: `title` "Update feed", `readOnlyHint:false`, `idempotentHint:true`
  (setting fields to a target state is idempotent).
- Instructions: "Updates an existing feed's settings. Only the provided fields
  are changed. Setting fields to explicit values is idempotent. Does not delete."
- Upstream: `PUT /v1/feeds/{feedID}`.

**`refresh_feed`** — force a refresh of one feed.
- Inputs: `feed_id` (`int`, required).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Refresh feed", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Triggers Miniflux to refresh this feed's entries. Repeated
  refresh is idempotent (Miniflux coalesces refreshes)."
- Upstream: `PUT /v1/feeds/{feedID}/refresh`.

**`create_category`** — create a category.
- Inputs: `title` (`string`, required).
- Outputs: the created category object (id, title).
- Annotations: `title` "Create category", `readOnlyHint:false`,
  `idempotentHint:true` (**search-before-create by title**).
- Instructions: "Creates a category with the given title. **Idempotent**: the
  handler first checks for an existing category with the same title and returns
  it instead of creating a duplicate. Safe to retry."
- Upstream: `POST /v1/categories`.
- **Idempotency mechanism (X02):** search-before-create on the category `title`
  (case-insensitive) against `list_categories`.

**`update_category`** — rename/reparent a category.
- Inputs: `category_id` (`int`, required), `title` (`string`, optional).
- Outputs: the updated category object.
- Annotations: `title` "Update category", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Updates a category's title. Setting a title is idempotent."
- Upstream: `PUT /v1/categories/{id}`.

**`refresh_category`** — force refresh of all feeds in a category.
- Inputs: `category_id` (`int`, required).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Refresh category", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Triggers a refresh of every feed in the category. Idempotent."
- Upstream: `PUT /v1/categories/{id}/refresh`.

**`mark_feed_entries_read`** — mark all entries of a feed as read.
- Inputs: `feed_id` (`int`, required).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Mark feed entries read", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Marks every entry in the feed as read. Setting read-state is
  idempotent — safe to retry."
- Upstream: `PUT /v1/feeds/{feedID}/mark-all-as-read`.

**`mark_category_entries_read`** — mark all entries of a category as read.
- Inputs: `category_id` (`int`, required).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Mark category entries read", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Marks every entry in the category as read. Idempotent."
- Upstream: `PUT /v1/categories/{id}/mark-all-as-read`.

**`update_entries`** — bulk set status/starred on a set of entries.
- Inputs: `entry_ids` (`int[]`, required, max 1000), `status`
  (`unread|read|removed`, optional), `starred` (`bool`, optional).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Update entries (bulk)", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Bulk-applies a status and/or starred flag to the given entry
  ids. Setting state to explicit values is idempotent. Prefer this over
  per-entry calls for batch operations."
- Upstream: `PUT /v1/entries` (body `{ entry_ids, status, starred }`).

**`toggle_entry_bookmark`** — toggle the starred/bookmark state of one entry.
- Inputs: `entry_id` (`int`, required).
- Outputs: the resulting `starred` (`bool`) state.
- Annotations: `title` "Toggle entry bookmark", `readOnlyHint:false`,
  **`idempotentHint:false`** (it toggles — see below).
- Instructions: "Flips the starred state of one entry. **Not idempotent** — each
  call toggles state, so calling twice returns it to the original value. Use
  `update_entries` with an explicit `starred` value when you want to set (not
  toggle) state."
- Upstream: `PUT /v1/entries/{entryID}/bookmark`.

**`update_entry`** — update an entry's title/content.
- Inputs: `entry_id` (`int`, required), `title` (`string`, optional),
  `content` (`string`, optional), `url` (`string`, optional).
- Outputs: the updated entry object.
- Annotations: `title` "Update entry", `readOnlyHint:false`,
  `idempotentHint:true`.
- Instructions: "Updates an entry's title/content/url. Setting fields to values
  is idempotent. Does not change read/starred state."
- Upstream: `PUT /v1/entries/{entryID}`.

**`import_opml`** — import subscriptions from an OPML document.
- Inputs: `opml` (`string`, required — the OPML XML document).
- Outputs: `feeds` (created feed ids).
- Annotations: `title` "Import OPML", `readOnlyHint:false`,
  `idempotentHint:true` (Miniflux returns `200` when an entry already exists).
- Instructions: "Imports feed subscriptions from OPML. **Idempotent**: Miniflux
  returns 200 for feeds that already exist, so re-importing the same OPML does
  not create duplicates. The document is sent to Miniflux and not stored
  locally."
- Upstream: `POST /v1/import`.
- **Idempotency mechanism (X02):** Miniflux natively returns `200` when an
  imported entry already exists — a genuine upstream idempotency guarantee, so
  `idempotentHint:true` is valid.

### 4.3 Delete / destructive tools (`readOnlyHint: false`, `destructiveHint: true`, HITL S12)

> Destructive tools are irreversible. **Human-in-the-loop (S12):** clients must
> confirm before these run; the server never auto-cascades destructive calls.
> All are `destructiveHint: true`.

**`delete_feed`** — permanently remove a feed subscription.
- Inputs: `feed_id` (`int`, required); `confirm` (`bool`, required — HITL).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Delete feed", `readOnlyHint:false`,
  `destructiveHint:true`, `idempotentHint:true` (deleting an already-deleted
  feed is a no-op).
- Instructions: "Permanently deletes a feed and its entries. **Irreversible —
  require human confirmation.** Delete is idempotent (deleting a missing feed
  succeeds)."
- Upstream: `DELETE /v1/feeds/{feedID}`.

**`delete_category`** — permanently remove a category.
- Inputs: `category_id` (`int`, required); `confirm` (`bool`, required — HITL).
- Outputs: `ok` (`bool`).
- Annotations: `title` "Delete category", `readOnlyHint:false`,
  `destructiveHint:true`, `idempotentHint:true`.
- Instructions: "Permanently deletes a category. **Irreversible — require human
  confirmation.** Deleting a missing category succeeds (idempotent)."
- Upstream: `DELETE /v1/categories/{id}`.

**`flush_history`** — purge history (removed/older entries) from Miniflux.
- Inputs: `confirm` (`bool`, required — HITL); optionally `before` RFC 3339
  timestamp.
- Outputs: `ok` (`bool`).
- Annotations: `title` "Flush history", `readOnlyHint:false`,
  `destructiveHint:true`, `idempotentHint:true`.
- Instructions: "Purges old/removed entry history from Miniflux. **Irreversible
  — require human confirmation.** Idempotent (flushing an already-clean history
  is a no-op)."
- Upstream: `PUT /v1/flush-history`.

### 4.4 Excluded from the tool surface (least privilege, S10)

The following are **intentionally NOT exposed** as tools:

- **User management** — create/delete/update users, passwords, roles, etc.
- **API-key management** — create/delete/invalidate Miniflux API keys.

Only the read-only `get_me` (current-user profile) is exposed. Rationale: the
MCP server operates with a scoped API key and must not be able to mutate
account-level principals or credentials (S10). This keeps the attack surface and
blast radius minimal.

### 4.5 Complete use-case coverage (M05/N10)

The tool set covers these end-to-end tasks without client scripting:

- **"What's in my queue?"** → `list_entries` (+ `get_counters`, `get_entry`).
- **"Subscribe to a new feed"** → `discover_subscriptions` →
  `create_feed` (idempotent).
- **"Catch up on a feed"** → `get_feed_entries` → `mark_feed_entries_read`.
- **"Organize subscriptions"** → `list_categories` → `create_category` /
  `update_feed` (category_id) / `delete_category`.
- **"Backup / migrate"** → `export_opml` → `import_opml`.
- **"Bookmark for later"** → `toggle_entry_bookmark` / `update_entries`.
- **"Clean up"** → `delete_feed` / `flush_history` (HITL).

No task forces the client to chain low-level/raw calls; each step is a named,
typed tool.

---

## 5. Protocol surface

The server implements the **tools** capability of MCP via the official Go SDK.
Capabilities and scope:

- **Tools** — implemented (the full set in §4).
- **Resources** — **not implemented**. Miniflux state is exposed only through
  tools, not as URI-addressed resources.
- **Prompts** — **not implemented** (no templated prompt workflows; the model
  drives the tools directly).
- **Sampling** — **not implemented** (the server never requests LLM sampling).
- **Roots** — **not implemented** (no local filesystem/roots; S04 N/A).
- **Logging capability (MCP)** — not exposed as an MCP capability; server
  logging is handled by the server's own logrus pipeline (§9) and SDK slog is
  wired into it (L07/L03GO).

The protocol handshake (`initialize`) is served by the go-sdk over the selected
transport (§3). JSON-RPC error mapping is defined in §6 (error taxonomy).

---

## 6. Architecture (A01, A02, C07GO)

### 6.1 Layout — top-level named packages (A01)

The Go layout is **always layered** (DDD/Clean). **Top-level named packages are
the default**; the `internal/` visibility wrapper is **NOT used** in this
project.

> **Decision & justification (A01 — internal/ not used):** `internal/` is
> omitted because there is **no concrete reason to restrict Go package
> visibility** here: this repo is a standalone server binary (not imported as a
> library by external code) and does not require strict cross-package
> visibility enforcement. Per A01, using `internal/` without such a
> justification MUST NOT happen — so we deliberately keep the top-level named
> packages. This decision is recorded here, not skipped.

```
cmd/mcp-miniflux/main.go      # composition root (thin): wires config→logger→layers→transports
domain/                       # innermost: pure types & ports (no deps)
  config.go                   #   Config struct (env-driven) + redaction-aware model
  miniflux/                   #   Miniflux domain models (Feed, Category, Entry, Counters, Me)
  tools/                      #   tool input/output structs (S02/S08/S09), inputSchema/outputSchema
application/                  # use-case handlers: one per tool, orchestrate domain + infra ports
  handlers/                   #   list_feeds.go, create_feed.go, ... (application layer)
  registry.go                 #   tool registry: declares + registers all tools on the go-sdk server
infrastructure/               # implements domain ports
  config.go                   #   envconfig loader (LoadConfig) -> domain.Config
  logging/                    #   logrus logger + slog->logrus adapter (L01GO/L03GO/L04GO)
  mcp/                        #   go-sdk server + transport wiring (Streamable HTTP)
  miniflux/                   #   Miniflux HTTP client (resty v3), auth header, redaction
  observability/              #   :8081 metrics+pprof+probes (O01/O02/O03), upstream metrics (O03)
```

Layer edges are **enforced by go-arch-lint** (C07GO) via `.go-arch-lint.yml`
(components `cmd`, `application`, `infrastructure`, `domain`; `deps`):
- `cmd` → `application`, `infrastructure` (wiring);
- `application` → `domain` (uses domain types/ports);
- `infrastructure` → `domain` (implements domain ports);
- `domain` → (nothing) — innermost leaf.

Every other edge (e.g. `application` → `infrastructure`, `domain` →
`application`) is forbidden and fails CI (C07GO). Third-party vendored
libraries (go-sdk, resty, logrus, prometheus, envconfig) are allowed from any
layer.

### 6.2 Tool registry (A01)

Tools are declared in `domain/tools` as typed input/output structs carrying
their JSON Schemas and MCP metadata (annotations + instructions, §4), and are
**registered** in `application/registry.go` onto the go-sdk `Server` at startup:

1. Each tool handler is a struct implementing a common `application.Handler`
   interface (`Name()`, `InputSchema()`, `OutputSchema()`, `Annotations()`,
   `Instructions()`, `Call(ctx, args) (Result, error)`).
2. The registry iterates the ordered list (read → write/update → delete, S03),
   wraps each in the go-sdk tool, and calls the SDK's tool-registration API.
3. The registry is injected with the Miniflux client (infrastructure) and the
   logger, so handlers never construct infrastructure themselves (DIP).

This keeps the tool surface declarative, discoverable, and centrally
enumerable.

### 6.3 Transport wiring (A01, M02)

- The go-sdk `Server` is created with `ServerOptions` (instructions from §4,
  `Logger` = the slog→logrus adapter, L03GO).
- The **Streamable HTTP** transport is selected and the server bound to
  `LISTEN_ADDR` (default `:8080`). In `-mode stdio` (debug only), the SDK's
  stdio transport is used instead.
- The observability server is bound to `INTERNAL_ADDR` (default `:8081`) as a
  separate `http.Server` (O01/O04), fully independent of the MCP JSON-RPC flow.

### 6.4 Config (A01, S02, S04)

Env-driven via `envconfig` (`infrastructure/config.go` → `domain.Config`).
Fields that may hold secrets are annotated `secret:true` (S02). Full table in
§9. `ALLOW_DIRS`/`ALLOW_SYMLINKS` are **N/A** — this server has **no local
filesystem access** (S04 N/A; documented).

### 6.5 Error handling taxonomy (A01, X03)

Errors are classified into a taxonomy and mapped to MCP / JSON-RPC codes:

| Category | Detection | MCP / JSON-RPC mapping | Upstream behavior |
|----------|-----------|------------------------|-------------------|
| **Validation** | tool args fail `inputSchema`, or a required field is absent | `InvalidParams` (-32602) | no upstream call |
| **Not-found** | upstream returns 404 | `NotFound` / `-32602`-adjacent (mapped to a typed not-found error) | returned to model, logged |
| **Transient** | upstream 5xx / 429 / network error, timeout | `InternalError` / application error (JSON-RPC error) | **returned to the model**; **no silent retry** (X03/N26) |
| **Auth** | upstream 401/403 | application error | returned to model, logged (no token in log, L05) |

- **No silent retry (X03/N26):** every upstream failure — including rate-limit
  `429` — is **returned to the model** and **logged** (no secrets, L05/L08);
  there is no retry/backoff layer (L02GO). Timeouts/contexts are explicit on
  every outbound call.
- The error is always logged with its `request_id` (§9) and a sanitized
  description; the token and request bodies are never logged (L05).

### 6.6 Startup — local wiring only (A02/N35)

Startup performs **local wiring only**:

1. Load config (envconfig).
2. Build the logrus logger (channel/level/format per L01/L02/L04).
3. Emit the **startup banner** as the very first log line (B05/L06).
4. Compose the layers (registry + handlers + clients).
5. Start the MCP transport (`:8080`) and the observability server (`:8081`).

The server **never makes an outbound/upstream request at startup** — no
connectivity/health ping to Miniflux, no eager data fetch, no credential/auth
validation call (A02/N35). Any apparent need to "check the upstream" is resolved
**lazily on the first tool call** (X01: a tool call maps to an upstream
request). The observability liveness probe therefore reflects process liveness,
**not** upstream reachability (which is observable via O03 upstream metrics).

---

## 7. Security (S01–S12)

- **S01/N01 — TLS never in-server.** TLS termination is the **reverse proxy's**
  job; the server listens on plain HTTP on `:8080` (MCP) and `:8081`
  (observability). No in-process TLS, no certificates.
- **S02/N02 — No secret leakage, annotation-based.** All Miniflux API
  requests, API responses and config fields are modeled as **structs**
  (`domain/miniflux/*`, `domain/config.go`). Any field that contains or may
  contain sensitive data is annotated **`secret:true`** — notably feed
  `password`, `MINIFLUX_API_TOKEN`, and any HTTP `Authorization`-equivalent
  material. A **single redaction helper** (`infrastructure/.../redact.go`)
  recursively strips annotated fields from **tool output and logs**. No
  heuristics/regex "guessing" — the struct is authoritative. This drives L05
  and the L08 access-log redaction.
- **S04/N03 — No filesystem access.** `ALLOW_DIRS`/`ALLOW_SYMLINKS` are **N/A**
  (no local filesystem). Documented, not silently skipped.
- **S05/N02GO — Fix, don't suppress.** Security-scanner findings (gosec,
  govulncheck, gitleaks) are **fixed**, never blanket-suppressed or excluded by
  default.
- **S06/N16 — Naming.** Public GitHub repo → standard public module path
  `github.com/teran/mcp-miniflux` (the internal/Forgejo local-only rule does
  not apply).
- **S07/N20 — Open-world trust boundary.** The single open-world tool
  `discover_subscriptions` treats its output as **untrusted data** and returns
  it **structurally** (`structuredContent` + `outputSchema`, typed candidates),
  never raw free-form text.
- **S08/N22 — Strict input validation.** Every `tools/call` validates
  arguments against `inputSchema` (`additionalProperties:false`, strict types,
  limits) **before** execution; invalid → `InvalidParams`. (See §4.)
- **S09/N23 — Output validation & sanitization.** Output conforms to each
  tool's `outputSchema`; ANSI/control escape sequences are filtered from text
  output.
- **S10 — Least privilege.** (a) User-management and API-key-management tools
  are **excluded** (§4.4) — the scoped API key cannot mutate principals. (b)
  The open-world tool `discover_subscriptions` is **isolated**: no `ALLOW_DIRS`,
  no secrets, no elevated scope (§4.1).
- **S11/N21 — No concatenation into shell/SQL/URL.** No shell execution; no
  SQL; outbound HTTP via **resty** (L02GO) with parameters set as fields, never
  string-concatenated URLs. For the open-world `discover_subscriptions`, the
  input `url` is validated to be an absolute `http`/`https` URL and is passed
  only to the trusted Miniflux instance; SSRF exposure is bounded to the
  configured Miniflux API (the sidecar has no other outbound surface).
- **S12 — HITL for destructive tools.** All `destructiveHint:true` tools
  (`delete_feed`, `delete_category`, `flush_history`) are human-confirmable and
  deniable; the server never auto-cascades destructive calls.
- **S13/N34 — Supply-chain (Go, lower risk than TS/JS).** Go module versions
  are pinned and current (C05), `go.sum` is committed, and gosec + govulncheck
  run in CI (C05GO/C06GO). No untrusted runtime packages are executed.
- **C03/N28 — gitleaks secret scan** over git history runs in CI (see §11),
  complementing runtime annotation-based redaction (S02).

---

## 8. Data & state (X01–X05)

- **X01/N24 — State model decision: STATELESS.** Each tool call maps 1:1 to one
  upstream Miniflux HTTP request (some idempotent handlers issue a preceding
  lookup, §4.2). **Miniflux holds all state** (feeds, categories, entries,
  counters, OPML). The server persists **no state** outside a single call and
  has no local storage, database or cache. There is therefore **no
  persistence, no at-rest encryption, no schema migration, and no concurrent
  file-write concern** (X04/X05 N/A — documented, not skipped).
- **X02/N25 — Write idempotency.** Every mutating tool has a real data-level
  mechanism before it claims `idempotentHint:true`:
  - `create_feed`, `create_category`: **search-before-create** (natural key —
    feed URL / category title), returning the existing object on duplicate.
  - `import_opml`: Miniflux returns `200` for already-existing entries
    (upstream idempotency guarantee).
  - state-setting tools (`update_feed`, `update_category`, `mark_*_read`,
    `update_entries`, `update_entry`, `refresh_*`): setting explicit state is
    idempotent.
  - `toggle_entry_bookmark`: **NOT idempotent** (toggles) — correctly
    `idempotentHint:false`.
- **X03/N26 — Upstream connection & limits.** Explicit per-call timeouts and
  contexts for all Miniflux calls (resty, L02GO). On failure (including `429`),
  the error is **returned to the model** and **logged** (no secrets); **no
  silent retry**.
- **X04/X05 — N/A (stateless).** No persistence, no files, no concurrency
  hazards. Declared explicitly per X01.

---

## 9. Logging (L01–L09, L01GO–L04GO)

- **L01/L01GO/N04GO — Channel matches transport (logrus).** HTTP mode → logs to
  **stdout** (12-factor). `-mode stdio` (debug) → logs to a **file**
  (`LOG_FILENAME`, chmod 600), never stdout.
- **L02 — Launch-mode enablement.** HTTP mode (`-mode http`, the deployed
  mode): logging **always enabled**, default level **`info`** (overridable via
  `LOG_LEVEL`). stdio mode (`-mode stdio`, default flag): logging enabled
  **only when `LOG_LEVEL` is set**; unset ⇒ disabled.
- **L03 — `LOG_FILENAME`** overrides the log path (stdio/file mode).
- **L04 — Default format text** (full absolute timestamp); `LOG_FORMAT=json`
  switches to JSON.
- **L05/N02 — No secrets in logs.** Driven by `secret:true` annotations (S02);
  the redaction helper strips secrets from all log records and tool access logs.
- **L06 — Banner first.** When logging is enabled (always in HTTP mode), the
  **B05 startup banner** is the **first line** written to the channel-appropriate
  log.
- **L07/L03GO — SDK logger wired.** The go-sdk `slog` logger is wired into
  logrus (`ServerOptions.Logger` = a forwarding slog handler), so SDK events are
  visible in the server logs.
- **L08/L03GO — Tool-call access log at `info`.** On each `tools/call`, a
  per-request log line at **`info`** (no `debug` gate — visible whenever logging
  is on) with structured fields: `tool`, **redacted** `args`, `source`,
  `duration`, `outcome` (`ok`/`error`). `source` is `STDIO` for a Local stdio
  server, or the upstream IP from `X-Real-IP`/`X-Forwarded-For` when a gateway
  populates them. No secrets.
- **L09/L04GO — `request_id` correlation.** A `request_id` is generated per
  request, carried in `context`, threaded on every log record, and propagated to
  the outbound Miniflux call as `X-Request-ID`; the outbound client (resty)
  emits its own record with the same `request_id`. Implemented via
  `WithRequestID`/`RequestIDFromContext` + context-aware logrus entry builder.

**Config summary (env-driven, S02):**

| Variable | Default | Description |
|----------|---------|-------------|
| `MINIFLUX_API_URL` | — | Miniflux base URL (e.g. `https://reader.example.com`) |
| `MINIFLUX_API_TOKEN` | — | Miniflux API token (`secret:true`) |
| `LISTEN_ADDR` | `:8080` | MCP app (Streamable HTTP) listen address (O04) |
| `INTERNAL_ADDR` | `:8081` | observability listen address (O04) |
| `LOG_LEVEL` | `info` (HTTP) | log level; in stdio mode, unset ⇒ disabled (L02) |
| `LOG_FORMAT` | `text` | `text` or `json` (L04) |
| `LOG_FILENAME` | — | log file for `-mode stdio` (L01/L03, chmod 600) |
| `ALLOW_DIRS` / `ALLOW_SYMLINKS` | N/A | no local filesystem (S04 N/A) |

**Startup banner (B05/L06), first line when logging enabled:**
`Starting {appName}/{appVersion} (commit: {appCommit}; built at {appTimestamp}) ...`

---

## 10. Metrics & Observability (O01–O04, N32)

- **O01/N32 — Internal observability endpoint (always present).** A Remote
  server MUST always expose it — **no opt-out**. Served on the **separate
  address `INTERNAL_ADDR` (default `:8081`)**, distinct from the MCP app
  address (`:8080`, O04). It serves **Prometheus metrics**, **pprof**, and the
  **startup / readiness / liveness probes**. Metrics and probes are **not** part
  of the MCP tool / JSON-RPC flow.
- **O02 — Default Go HTTP metrics.** The endpoint exposes the standard Go HTTP
  metrics via `promhttp` default collectors: Go runtime/memstats + `net/http`
  per-request counters and histograms for **latency, request/response size and
  response status codes**.
- **O03 — Upstream (Miniflux) response metrics.** Because this is a
  **proxying/passthrough** server, the endpoint also exposes **upstream
  response metrics by analogy**: an upstream **latency** histogram, **request/
  response size**, and a **status-code** counter labelled by upstream status —
  so the Miniflux path is observable alongside the server's own metrics (this is
  how "is Miniflux reachable/healthy" is answered, without any startup ping,
  A02).
- **O04 — Addresses.** MCP app listen default `:8080` (`LISTEN_ADDR`);
  internal observability default `:8081` (`INTERNAL_ADDR`); **both
  env-overridable**. Because observability is on a different address, a reverse
  proxy forwards **only** the MCP transport to `:8080`.

Probe semantics: **liveness/readiness** reflect the process and listener state
(local only); **upstream health** is observable via O03 metrics, not a
startup-time or probe-time ping (A02/N35).

---

## 11. CI/CD (R01–R07, C01–C06, C01GO–C08GO, C09GO N/A, B01–B05, T01)

### 11.1 Build-system interface (R07/N31)

The Makefile exposes the standard, language-agnostic interface that CI binds to
(no raw `go ...` calls in CI):

- `make lint` — golangci-lint (incl. gofmt/gofumpt), `go vet`, gosec,
  govulncheck, go-arch-lint (C03GO/C05GO/C06GO/C07GO).
- `make test` — `go test -race` + **coverage ≥ 95% gate** (C01/C04GO), failing
  the build below 95%.
- `make build` — **goreleaser** → single binary artifact per platform into
  `dist/` (B01/B02/B04).
- `make container-image` / `make container-image push=true` — build-only /
  build+push the image **from the `build` artifact** (B04) — declared because
  this is a **Remote** server (R01/B03).

**Dedicated hard-gate jobs** (not part of the interface, per R07/N31): **gremlins**
mutation (C02/C08GO) and **gitleaks** secret scan (C03/N28).

### 11.2 CI pipeline (GitHub Actions, R06) — 4 stages

**Stages:** linters → build → tests → release artifacts. All bound to
`make <target>` (R07/N31); no job invokes an undeclared optional target.

- **Linters (lint):** `make lint` (golangci-lint + vet + gosec + govulncheck +
  go-arch-lint). Findings **fixed, not suppressed** (C05GO/C06GO/N02GO).
- **Mutation gate (C02/C08GO/N03GO):** dedicated hard gate —
  `gremlins unleash ./... --threshold-efficacy=80 --threshold-mcover=80`,
  **fails the build** below 80% (no continue-on-error / badge-placeholder).
- **Secret scan (C03/N28):** `gitleaks detect --source . --redact --verbose`
  over **git history**; findings **fixed**, not suppressed.
- **Build:** `make build` (goreleaser).
- **Tests:** `make test` — `go test -race` + **coverage ≥ 95% gate** (C01/N06,
  fails below 95). No e2e job: this project has **no e2e suite** (C04/T02/C09GO
  N/A, §11.5). Unit + coverage (C01) + mutation (C02) gates apply.
- **Release artifacts:** on tags → **goreleaser release** (B01) publishing the
  binary artifacts; then `make container-image push=true` (R01/B03/B04) building
  and pushing the image **from the single release binary** (B04/N18).
- **Live badge publishing (`badges` job) — documented reporting-only exception
  (NOT an N31 violation).** The `badges` job re-computes the quality metrics for
  the README's shields.io/endpoint badges by invoking the underlying tools
  **directly** (raw `go test`/`go tool cover`, `gosec`, `govulncheck`,
  `gremlins`) rather than `make <target>`. This is a **deliberate, documented
  exception to the R07/N31 build-system interface**: the job is **reporting-only**
  — it recomputes metrics solely to render badge colors, is **gated on the real
  quality jobs** (`needs: [lint, build, test, mutation, secret-scan]`), pushes
  **only to the dedicated `badges` branch** (never `master`), and its pass/fail
  is **never a gate** on the build (it does not gate the pipeline or the release;
  it is not a required check). Because it is not a gate, invoking raw commands
  there does not violate N31 (which prohibits CI *gating* on language-specific
  commands instead of the make interface). The actual quality gates remain the
  make-bound jobs above.

### 11.3 Version currency (C05/N33, C01GO)

The project pins **latest-stable** versions (single source of truth
`reference/versions.md`):

| Component | Version |
|-----------|---------|
| Go | **1.27.1** (`go 1.27.1` in `go.mod`; CI `GO_VERSION: "1.27"`) (C01GO/N01GO) |
| MCP Go SDK | **v1.8.0** (`github.com/modelcontextprotocol/go-sdk`) (M01) |
| resty.dev/v3 | **v3.0.0-rc.4** (L02GO; no stable yet — latest available) |
| prometheus client_golang | **v1.24.1** (O01–O04) |
| logrus | **v1.10.2** (L01GO) |
| envconfig | **v1.4.0** (env-driven config) |
| jsonschema-go | **v0.4.3** (`github.com/google/jsonschema-go`, direct dep — input/output schema generation, S08/S09) |

Staleness is a **defect** at conformance (C05/N33): upgrade/fix, never suppress.

### 11.4 Release & image rules (R01–R04, B01–B05)

- **R02:** default branch `master`.
- **R01/B03:** a Remote server **MUST** build & publish a container image via
  CI/CD — yes (release stage).
- **R03 (git tag `X`):** image tags `X`, `X-{ts}`, `X-{commit}`,
  `X-{commit}-{ts}`.
- **R04 (commit to `master`):** image tags `master-{commit}`, `master-{ts}`,
  `master-{commit}-{ts}`.
- **B01:** binary release artifacts from a dedicated release step (goreleaser).
- **B02:** binary embeds build metadata via ldflags — `appName`, `appVersion`,
  `appCommitHash`, `appTimestamp`.
- **B04/N18:** the image is built **from the single release binary artifact**
  (distroless nonroot Dockerfile; no in-image compilation).
- **B05:** when logging is enabled (always in HTTP mode), the **startup banner**
  is the first log line (§9).

**Dockerfile (B04):** multi-stage, **distroless `nonroot`** (UID 65532),
per-platform `COPY` of the release binary via `TARGETARCH` (linux/amd64,
linux/arm64), `EXPOSE 8080`, `ENTRYPOINT ["/app/mcp-server", "-mode", "http"]`
— no TLS in-process (N01/S01; reverse proxy terminates), logs to stdout (L01).

**B02 — embedded `appVersion` vs image tag (documented behaviour).** `make
build` runs **goreleaser snapshot** (`goreleaser build --snapshot`), which
labels the artifact version as a **snapshot**, not the git tag. The image is
built **from that snapshot binary** (`make container-image` → `make build`),
so for an untagged `master` build the binary's embedded `appVersion` is the
snapshot string while the image carries the `master-{commit}` tag set (R04).
For a **tagged release**, the intent is that the embedded `appVersion` equals
the image's git tag `X` (R03): the release job should propagate the tag into
the build so `goreleaser` embeds `{{ .Version }} == X` (e.g. via
`GORELEASER_CURRENT_TAG`/a `VERSION` variable passed through `make
container-image`). Until that wiring is in place, a TODO in the release
workflow tracks it; the startup banner (§9, B05) therefore reports the
embedded version, which may lag the image tag on snapshot builds — this is
expected and not a defect in the binary.

### 11.5 e2e tests — N/A (T02/C04/C09GO/N07GO)

This project has **no e2e tests** by decision: a hermetic, deterministic
full-stack e2e against the Miniflux upstream cannot be guaranteed in CI (the
upstream's runtime behaviour — refresh, discovery — is not deterministic
enough), so we rely on unit tests (coverage ≥ 95%, C01), mutation testing
(C08GO) and the static/secret gates (C03/C05GO/C06GO). Consequently
**C04/T02/C09GO/N07GO/N30 are N/A**: there is no e2e suite to run, and **no
`make e2e` target and no e2e CI job are declared** (R07/N31).

### 11.6 TDD workflow (T01)

Development follows TDD: **@qa** writes the tests first (isolated context) and
**@developer** writes the implementation (isolated context), per the skill's
workflow.

---

## 12. Docs (D01–D04)

- **D02/N04 — SPEC.md and AGENTS.md strictly English.** This document is in
  English.
- **D01/N05 — README in English** by default.
- **D03/N11 — README begins with the AI-Generated Content disclaimer** as the
  first line.
- **D04/N12 — README includes the required badge set** directly under the title
  (CI/build, release, license, MCP, and Go badges).

---

## Appendix A — MUST/MUST NOT conformance trace

The following **MUST** criteria are satisfied in this SPEC; the section column
points to where each is addressed:

| Code | Addressed in |
|------|--------------|
| M01 | §2, §11.3 (go-sdk v1.8.0) |
| M02 | §3 |
| M03 | §3 |
| M04 | §4 (annotations + instructions per tool) |
| M05 | §4.5 |
| M06 | §3 |
| M07 | §4 (no mega-tool) |
| C01 | §11.2 (`make test` coverage ≥ 95) |
| C02 | §11.2 (gremlins 80/80 hard gate) |
| C03 | §7, §11.2 (gitleaks) |
| C04 | N/A — no e2e suite (§11.5) |
| C05 | §11.3 |
| T01 | §11.6 |
| T02 | N/A — no e2e suite (§11.5) |
| S01 | §7 |
| S02 | §7 |
| S03 | §4 |
| S04 | §6.4, §7 (N/A — no filesystem) |
| S05 | §7 |
| S06 | §7 |
| S07 | §7, §4.1 |
| S08 | §4 |
| S09 | §4 |
| S10 | §7, §4.4 |
| S11 | §7 |
| S12 | §4.3, §7 |
| R01 | §2, §11.4 |
| R02 | §2 |
| R03 | §11.4 |
| R04 | §11.4 |
| R05 | §2 |
| R06 | §2, §11.2 |
| R07 | §11.1 |
| B01 | §11.4 |
| B02 | §11.4 |
| B03 | §11.4 |
| B04 | §11.4 |
| B05 | §9, §11.4 |
| L01 | §9 |
| L02 | §9 |
| L03 | §9 |
| L04 | §9 |
| L05 | §9 |
| L06 | §9 |
| L07 | §9 |
| L08 | §9 |
| L09 | §9 |
| A01 | §6 |
| A02 | §6.6 |
| D01–D04 | §12 |
| X01 | §8 |
| X02 | §8, §4.2 |
| X03 | §6.5, §8 |
| X04 | N/A — stateless (§8) |
| X05 | N/A — stateless (§8) |
| O01–O04 | §10 |
| C01GO–C09GO | §11, §6.1 (C09GO N/A — no e2e, §11.5) |
| L01GO–L04GO | §9 |

No **MUST NOT** is violated. Each `MUST NOT` (N01–N35, N01GO–N07GO) is either
addressed (the requirement that would be violated is satisfied elsewhere) or
explicitly N/A — traced per code below.

**Common MUST NOT (N01–N35):**

| Code | Addressed / N/A |
|------|-----------------|
| N01 | §7 (TLS never in-server — S01) |
| N02 | §7 (no secret leakage — S02) |
| N03 | §6.4, §7 (N/A — no filesystem — S04) |
| N04 | §12 (SPEC/AGENTS English — D02) |
| N05 | §12 (README English — D01) |
| N06 | §11.2 (coverage ≥ 95 hard gate — C01) |
| N07 | §6 (application-level architecture — A01) |
| N08 | §3 (transport/OAuth2 justifications — M02/M03) |
| N09 | §4 (per-tool annotations + instructions — M04) |
| N10 | §4.5 (complete use cases — M05) |
| N11 | §12 (README AI-Generated Content disclaimer — D03) |
| N12 | §12 (README required badge set — D04) |
| N13 | §2 (default branch `master` — R02) |
| N14 | §11.4 (Remote image build via CI/CD — R01) |
| N15 | §11.2 (mutation testing hard gate — C02) |
| N16 | §7 (public module path — S06) |
| N17 | §11.4 (image only for Remote — B03) |
| N18 | §11.4 (single release binary for image — B04) |
| N19 | §2 (repo host/path decided before dev — R05) |
| N20 | §7, §4.1 (open-world structural output — S07) |
| N21 | §7 (no shell/SQL/URL concatenation — S11) |
| N22 | §4 (input validation against `inputSchema` — S08/S09) |
| N23 | §4 (ANSI/control sanitization — S09) |
| N24 | §8 (state model declared — X01/X04) |
| N25 | §8, §4.2 (data-level idempotency mechanism — X02) |
| N26 | §6.5, §8 (no silent retry, error logged — X03) |
| N27 | §8 (N/A — stateless, no file writes — X05) |
| N28 | §7, §11.2 (gitleaks over git history — C03) |
| N29 | §4 (no generic mega-tools — M07) |
| N30 | N/A — no e2e suite (§11.5) |
| N31 | §11.1, §11.2 (build-system interface — R07; badges exception documented) |
| N32 | §10 (observability endpoint always present — O01) |
| N33 | §11.3 (version currency — C05) |
| N34 | §7 (supply-chain, Go lower-risk — S13) |
| N35 | §6.6 (no outbound/upstream request at startup — A02) |

**Go MUST NOT (N01GO–N07GO):**

| Code | Addressed / N/A |
|------|-----------------|
| N01GO | §11.3 (latest stable Go pinned — C01GO) |
| N02GO | §7, §11.2 (scanner findings fixed, not suppressed — C05GO/C06GO) |
| N03GO | §11.2 (gremlins as a hard gate — C08GO) |
| N04GO | §9 (logrus; stdio logs to file, not stdout — L01GO) |
| N05GO | §7, §8 (outbound HTTP via resty v3, no low-level `http.Client` — L02GO) |
| N06GO | §9 (SDK slog wired into logrus; no secrets in trace lines — L03GO/L08) |
| N07GO | N/A — no e2e suite (§11.5) |
