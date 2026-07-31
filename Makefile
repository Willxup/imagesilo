SHELL := /bin/sh
GO_PACKAGES := ./cmd/... ./db/... ./internal/... ./tests/...

.PHONY: build check clean docker-build e2e format generate generate-check test web-build web-install web-sync

build: web-sync
	mkdir -p bin
	go build -trimpath -o bin/imagesilo ./cmd/imagesilo

check: web-sync
	test -z "$$(gofmt -l cmd db internal tests)"
	go vet $(GO_PACKAGES)
	go test $(GO_PACKAGES)
	bash -n scripts/*.sh
	npm --prefix web run lint
	npm --prefix web run typecheck
	npm --prefix web run test

clean:
	go clean
	rm -rf bin coverage internal/webui/dist web/dist web/playwright-report web/test-results

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 --file deploy/docker/Dockerfile .

e2e: build
	npm --prefix web run e2e

format:
	gofmt -w cmd db internal
	npm --prefix web exec -- eslint . --fix

generate:
	npm --prefix web run generate:api

generate-check:
	@temporary="$$(mktemp)"; \
	trap 'rm -f "$$temporary"' EXIT; \
	npm --prefix web exec -- openapi-typescript api/openapi.yaml -o "$$temporary"; \
	if ! cmp -s web/src/generated/openapi.d.ts "$$temporary"; then \
		diff -u web/src/generated/openapi.d.ts "$$temporary" || true; \
		echo 'Generated OpenAPI types are stale; run make generate.' >&2; \
		exit 1; \
	fi

test: web-sync
	go test $(GO_PACKAGES)
	npm --prefix web run test

web-build: generate-check
	npm --prefix web run build

web-sync: web-build
	rm -rf internal/webui/dist
	mkdir -p internal/webui/dist
	cp -R web/dist/. internal/webui/dist/

web-install:
	npm --prefix web install
