GO ?= go
BUN ?= bun
GORELEASER ?= goreleaser

BIN ?= ./bin/mangashelf
TAG ?=

.PHONY: build clean dev ensure-release-tag release test

build:
	mkdir -p $(dir $(BIN))
	$(GORELEASER) build --snapshot --clean --single-target --output $(BIN)

clean:
	rm -rf ./bin ./dist ./tmp ./web/dist ./extension/bundles ./coverage.out ./*.tsbuildinfo ./web/*.tsbuildinfo ./extension/*.tsbuildinfo

dev:
	$(BUN) install; \
	MANGASHELF_DEV_WEB_URL=http://127.0.0.1:$${MANGASHELF_DEV_WEB_PORT:-5173} \
	MANGASHELF_DEV_EXTENSION_URL=http://127.0.0.1:$${MANGASHELF_DEV_EXTENSION_PORT:-38181} \
	$(GO) tool air -c .air.toml & backend=$$!; \
	MANGASHELF_DEV_WEB_PORT=$${MANGASHELF_DEV_WEB_PORT:-5173} $(BUN) run web:dev & frontend=$$!; \
	MANGASHELF_DEV_EXTENSION_PORT=$${MANGASHELF_DEV_EXTENSION_PORT:-38181} $(BUN) run extension:dev & extension=$$!; \
	trap 'kill $$backend $$frontend $$extension 2>/dev/null' INT TERM EXIT; \
	wait

release: ensure-release-tag
	GITHUB_TOKEN="$$(printf 'url=%s\n\n' "$$(git remote get-url origin)" | git credential fill | sed -n 's/^password=//p')" \
	GORELEASER_FORCE_TOKEN=github \
	$(GORELEASER) release --clean

ensure-release-tag:
	@test -n "$(TAG)" || (echo "usage: make release TAG=v0.1.0" && exit 1)
	@if git rev-parse -q --verify "refs/tags/$(TAG)" >/dev/null; then \
		test "$$(git rev-list -n 1 "$(TAG)")" = "$$(git rev-parse HEAD)" || \
			(echo "tag $(TAG) already exists but does not point at HEAD" && exit 1); \
		echo "using existing tag $(TAG)"; \
	else \
		git tag -a "$(TAG)" -m "$(TAG)"; \
	fi

test:
	$(GO) test ./...
