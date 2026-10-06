# syntax=docker/dockerfile:1
#
# Dockerfile — SPEC §11.4 (B04/N18)
#
# The binary is produced by `make build` (goreleaser) into dist/ and copied
# into the image per TARGETARCH. There is NO in-image compilation. The runtime
# is `FROM scratch` (mirrors the fleet pattern from mcp-netbox /
# mcp-paperless-ngx): no shell, no TLS in-process (S01/N01 — reverse proxy
# terminates TLS), logs to stdout (L01). CA certificates are staged from an
# alpine base because the server makes outbound HTTPS calls to Miniflux, and a
# minimal /etc/passwd supplies the non-root user (65534 nobody).
# Usage (fleet pattern):
#   goreleaser build --snapshot --clean          (== make build)
#   cp dist/mcp-server_linux_amd64_v1/mcp-server mcp-server-linux-amd64
#   cp dist/mcp-server_linux_arm64_v8.0/mcp-server mcp-server-linux-arm64
#   docker buildx build --platform linux/amd64,linux/arm64 -t image:tag .

# Stage 1 — base: CA certificates + minimal passwd for the non-root user.
FROM alpine:3.24 AS base
RUN apk add --no-cache ca-certificates && \
    echo 'nobody:x:65534:65534:nobody:/:/sbin/nologin' > /etc/passwd-minimal

# Stage 2 — runtime: scratch, static, non-root.
FROM scratch
ARG TARGETARCH
COPY --from=base /etc/passwd-minimal /etc/passwd
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY mcp-server-linux-${TARGETARCH} /mcp-server

USER 65534:65534

# Streamable HTTP (MCP) listener (LISTEN_ADDR default :8080) + internal
# observability (:8081 — metrics/pprof/probes) on INTERNAL_ADDR.
EXPOSE 8080
EXPOSE 8081

ENTRYPOINT ["/mcp-server", "-mode", "http"]

LABEL org.opencontainers.image.source="https://github.com/teran/mcp-miniflux"
LABEL org.opencontainers.image.description="Remote MCP server for Miniflux"
LABEL org.opencontainers.image.licenses="Apache-2.0"
