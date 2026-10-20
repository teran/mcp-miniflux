# Security Policy

This MCP server is developed and tested largely by AI agents with minimal human
review, so security is enforced automatically by CI (lint, coverage, mutation,
gosec, govulncheck, gitleaks). The security baseline is described in `SPEC.md`
(S01–S13 and the domain-specific audit S14) — the single source of truth for
the security contracts this repository must honour.

## Reporting a Vulnerability

Please **do not** open a public issue for a security vulnerability.

Report vulnerabilities privately, preferably through GitHub's private
vulnerability reporting (Security → Report a vulnerability):
<https://github.com/teran/mcp-miniflux/security/advisories/new>

Alternatively, contact the maintainers directly at
me@teran.dev.

Please include:

- the affected version(s) and how you detected the issue;
- a minimal, reproducible description of the vulnerability;
- the potential impact and, if known, a suggested fix;
- the configuration context in which it occurs (e.g. `MINIFLUX_API_URL`,
  `X-Auth-Token` pass-through).

## What happens next

1. The maintainers acknowledge the report within 5 business days.
2. The issue is triaged: severity, affected versions, and a fix plan.
3. A fix is developed **test-first** and lands through the normal CI gates.
4. Once a fix is available, the vulnerability is disclosed and (if appropriate)
   a `v*` release is published.

We ask that you **do not** disclose the vulnerability publicly until a fix has
been released or the maintainers explicitly agree otherwise (responsible
disclosure).

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest release (`v*`) | ✅ |

Only the latest release line is supported. Older versions are not patched;
upgrade to a supported version.

## Security Baseline

- **TLS** is never implemented inside the server — it is terminated at the
  reverse proxy.
- **Secrets** are never leaked into tool output or logs (annotation-based
  redaction, `secret:true`).
- **Filesystem** — this server has **no local filesystem access** (S04 N/A);
  `ALLOW_DIRS`/`ALLOW_SYMLINKS` are not applicable.
- **Open-world tools** treat their output as untrusted data and are isolated
  from secrets and elevated scope (`discover_subscriptions`).
- **Destructive tools** require human confirmation (HITL).
- **Tokens** are passed straight through as `X-Auth-Token` and never logged.

See `SPEC.md` for the full domain-specific security audit (S14).
