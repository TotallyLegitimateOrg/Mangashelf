SQLC_VERSION ?= v1.31.1
AIR_VERSION ?= v1.65.1
BUN ?= bun
GO ?= go
VERSION ?= dev
COMMIT ?= unknown
BUILT_AT ?= unknown
BUILDINFO_PKG := github.com/TotallyLegitimateOrg/Mangashelf/internal/buildinfo
LDFLAGS := -X $(BUILDINFO_PKG).Version=$(VERSION) -X $(BUILDINFO_PKG).Commit=$(COMMIT) -X $(BUILDINFO_PKG).BuiltAt=$(BUILT_AT)

.PHONY: bootstrap sqlc build-web build-extension build dev run test clean

bootstrap:
	./scripts/bootstrap.sh

sqlc:
	$(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

build-web:
	$(BUN) run build:web

build-extension:
	$(BUN) run build:extension

build: bootstrap sqlc build-web build-extension
	mkdir -p bin
	$(GO) build -tags release -trimpath -ldflags '$(LDFLAGS)' -o ./bin/mangashelf ./cmd/mangashelf

dev:
	./scripts/dev.sh

run: build
	./bin/mangashelf

test: bootstrap
	$(GO) test ./...
	$(BUN) run build:web
	$(BUN) run test:extension


clean:
	rm -rf bin tmp web/dist extension/bundles web/tsconfig.app.tsbuildinfo
