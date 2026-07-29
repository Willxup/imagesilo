SHELL := /bin/sh
GO_PACKAGES := ./cmd/... ./db/... ./internal/...

.PHONY: build check clean docker-build e2e format generate test web-build web-install web-sync

build: web-sync
	mkdir -p bin
	go build -trimpath -o bin/imagesilo ./cmd/imagesilo

check: web-sync
	test -z "$$(gofmt -l cmd db internal)"
	go vet $(GO_PACKAGES)
	go test $(GO_PACKAGES)
	npm --prefix web run lint
	npm --prefix web run typecheck
	npm --prefix web run test

clean:
	go clean
	npm --prefix web run build -- --emptyOutDir

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 --file deploy/docker/Dockerfile .

e2e: build
	npm --prefix web run e2e

format:
	gofmt -w cmd db internal
	npm --prefix web exec -- eslint . --fix

generate:
	npm --prefix web run generate:api

test: web-sync
	go test $(GO_PACKAGES)
	npm --prefix web run test

web-build: generate
	npm --prefix web run build

web-sync: web-build
	rm -rf internal/webui/dist
	mkdir -p internal/webui/dist
	cp -R web/dist/. internal/webui/dist/

web-install:
	npm --prefix web install
