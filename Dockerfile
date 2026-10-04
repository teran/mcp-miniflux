# syntax=docker/dockerfile:1
#
# Dockerfile — SPEC §11.4 (B04/N18)
#
# The binary is produced by `make build` (goreleaser) into dist/ and copied
# into the image per TARGETARCH. There is NO in-image compilation. The runtime
# is distroless `nonroot` (UID 65532): no shell, no TLS in-process (S01/N01 —
# reverse proxy terminates TLS), logs to stdout (L01).

# Stage 1 — artifact holder: the release binary from the build context.
# goreleaser emits dist/mcp-server_linux_<arch>_<vN>/mcp-server (arch-version
# suffix differs per arch), so the source is matched with a wildcard.
FROM scratch AS artifact
ARG TARGETARCH
COPY dist/mcp-server_linux_${TARGETARCH}_*/mcp-server /app/mcp-server

# Stage 2 — runtime: distroless nonroot (UID 65532), static.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=artifact /app/mcp-server /app/mcp-server

# Streamable HTTP (MCP) listener (LISTEN_ADDR default :8080). Observability
# (:8081) is not exposed here — a reverse proxy forwards only :8080 (O04).
EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/app/mcp-server", "-mode", "http"]
