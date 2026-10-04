# Makefile — build-system interface (SPEC §11.1, R07/N31)
#
# CI binds ONLY to these targets (make lint / make test / make build /
# make container-image [push=true]). No raw `go ...` calls in CI.
# Dedicated hard gates (gremlins, gitleaks) are separate CI jobs, NOT part of
# this interface (R07/N31).

APP_NAME  ?= mcp-miniflux
BINARY    ?= mcp-server
GO        ?= go

# Container image (SPEC §11.4 R01/B03/B04)
IMAGE_NAME ?= ghcr.io/teran/mcp-miniflux
IMAGE_TAG  ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
PLATFORMS  ?= linux/amd64,linux/arm64

.PHONY: help lint test build container-image tidy fmt clean

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
	goreleaser build --snapshot --clean

container-image: build ## build (or build+push when push=true) distroless image from the release binary
	@if [ "$(push)" = "true" ]; then \
		echo "==> Building and pushing $(IMAGE_NAME):$(IMAGE_TAG) for $(PLATFORMS)"; \
		docker buildx build --platform $(PLATFORMS) --push -t $(IMAGE_NAME):$(IMAGE_TAG) .; \
	else \
		echo "==> Building $(IMAGE_NAME):$(IMAGE_TAG) for $(PLATFORMS) (push=true to publish)"; \
		docker buildx build --platform $(PLATFORMS) -t $(IMAGE_NAME):$(IMAGE_TAG) .; \
	fi

tidy: ## go mod tidy (helper)
	$(GO) mod tidy

fmt: ## gofmt + gofumpt (helper)
	$(GO) fmt ./...
	gofumpt -w .

clean: ## remove build/test artifacts
	rm -rf dist/ coverage.out bin/
