# Makefile — build-system interface (SPEC §11.1, R07/N31)
#
# CI binds ONLY to these targets (make lint / make test / make build /
# make container-image [push=true]). No raw `go ...` calls in CI.
# Dedicated hard gates (gremlins, gitleaks) are separate CI jobs, NOT part of
# this interface (R07/N31).

APP_NAME  ?= mcp-miniflux
BINARY    ?= mcp-server
GO        ?= go
# Real release version to embed as appVersion (R03/B02). Empty by default ->
# `make build` keeps goreleaser's default snapshot version. CI sets this to the
# git tag (tagged release) or the master-{commit} image tag so the container
# image binary embeds the real version, not a bare snapshot string. Forwarded to
# goreleaser via GORELEASER_CURRENT_TAG (there is no --build-version flag).
VERSION   ?=

# Container image (SPEC §11.4 R01/B03/B04, R03/R04 multi-tag scheme)
IMAGE_NAME ?= ghcr.io/teran/mcp-miniflux
IMAGE_TAG  ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
# Reusable tag building blocks (R03/R04). CI overrides IMAGE_TAG / EXTRA_TAGS.
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
TS         ?= $(shell date -u +%Y%m%d%H%M%S)
# Comma-separated ADDITIONAL image tags (beyond IMAGE_TAG) to build/push, e.g.
#   tag X   : "X-{ts},X-{commit},X-{commit}-{ts}"   (R03 — 4-tag set)
#   master  : "master-{ts},master-{commit}-{ts}"    (R04 — 3-tag set)
EXTRA_TAGS ?=
PLATFORMS  ?= linux/amd64,linux/arm64
comma      := ,

.PHONY: help lint test build release container-image tidy fmt clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

lint: ## golangci-lint + vet + gosec + govulncheck + go-arch-lint (findings FIXED, never suppressed)
	golangci-lint run ./...
	$(GO) vet ./...
	gosec ./...
	govulncheck ./...
	go-arch-lint check

test: ## go test -race with coverage >= 95% gate (C01/C04GO) — fails below 95%
	$(GO) test -race ./... -coverprofile=coverage.out
	@$(GO) tool cover -func=coverage.out | awk '/^total:/ { cov = $$3 + 0; if (cov < 95.0) { printf "FAIL: total coverage %.2f%% < 95%%\n", cov; exit 1 } else { printf "PASS: total coverage %.2f%%\n", cov } }'

build: ## goreleaser -> single binary per platform into dist/ (B01/B02/B04)
	$(if $(VERSION),GORELEASER_CURRENT_TAG=$(VERSION) )goreleaser build --snapshot --clean

release: ## goreleaser release --clean -> publish binary artifacts to GitHub Release (B01/B02)
	goreleaser release --clean

container-image: build ## build (or build+push when push=true) scratch image from the release binary
	cp dist/mcp-server_linux_amd64_v1/mcp-server mcp-server-linux-amd64
	cp dist/mcp-server_linux_arm64_v8.0/mcp-server mcp-server-linux-arm64
	@set -e; \
	echo "==> Checking binaries are statically linked (required for FROM scratch)"; \
	for b in mcp-server-linux-amd64 mcp-server-linux-arm64; do \
		if ! file "$$b" | grep -q 'statically linked'; then \
			echo "ERROR: $$b is NOT statically linked (is CGO_ENABLED=0?); cannot run on FROM scratch"; \
			exit 1; \
		fi; \
		echo "   OK: $$b -> $$(file -b "$$b" | sed 's/,.*//')"; \
	done; \
	TAGS="-t $(IMAGE_NAME):$(IMAGE_TAG)"; \
	TAGS="-t $(IMAGE_NAME):$(IMAGE_TAG)"; \
	for t in $(subst $(comma), ,$(EXTRA_TAGS)); do \
		[ -n "$$t" ] && TAGS="$$TAGS -t $(IMAGE_NAME):$$t"; \
	done; \
	if [ "$(push)" = "true" ]; then \
		echo "==> Building and pushing $(PLATFORMS) image tags: $(IMAGE_NAME):$(IMAGE_TAG), $(EXTRA_TAGS)"; \
		docker buildx build --platform $(PLATFORMS) --push $$TAGS .; \
	else \
		echo "==> Building $(PLATFORMS) image tags: $(IMAGE_NAME):$(IMAGE_TAG), $(EXTRA_TAGS) (push=true to publish)"; \
		docker buildx build --platform $(PLATFORMS) $$TAGS .; \
	fi

tidy: ## go mod tidy (helper)
	$(GO) mod tidy

fmt: ## gofmt + gofumpt (helper)
	$(GO) fmt ./...
	gofumpt -w .

clean: ## remove build/test artifacts
	rm -rf dist/ coverage.out bin/ mcp-server-linux-amd64 mcp-server-linux-arm64
