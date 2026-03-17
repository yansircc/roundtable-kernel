SHELL := /bin/bash

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)'
BINDIR ?= $(shell if [ -n "$$(go env GOBIN)" ]; then printf '%s' "$$(go env GOBIN)"; else printf '%s/bin' "$$(go env GOPATH)"; fi)

.PHONY: help build install test test-unit test-integration build-ui ci

help:
	@printf '%s\n' \
		'make build     Build ./rtk with version metadata.' \
		'make install   Install rtk into $$(go env GOBIN || GOPATH/bin).' \
		'make test      Run all Go tests.' \
		'make test-unit Run unit tests.' \
		'make test-integration Run integration tests.' \
		'make build-ui  Build the Svelte UI bundle under ui/dist.' \
		'make ci        Run unit tests, integration tests, build, and UI build.'

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o rtk ./cmd/rtk

install:
	mkdir -p "$(BINDIR)"
	go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BINDIR)/rtk" ./cmd/rtk

test:
	go test ./...
	go test -tags=integration ./...

test-unit:
	go test ./...

test-integration:
	go test -tags=integration ./...

build-ui:
	npm ci --prefix ui
	npm run build --prefix ui

ci: test-unit test-integration build build-ui
