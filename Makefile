BINARY_NAME=packer-plugin-proxmox-lxc
PLUGIN_FQN="$(shell grep -E '^module' <go.mod | sed -E 's/module *//')"
GO?=go
GOFMT?=gofmt
GOLINT?=golangci-lint

BUILD_TIME?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags="-s -w -X github.com/leoarry/packer-plugin-proxmox-lxc/version.Version=0.1.0 -X github.com/leoarry/packer-plugin-proxmox-lxc/version.BuildTime=$(BUILD_TIME)"

# Packer plugin installation directory
PACKER_PLUGIN_DIR?=$(HOME)/.config/packer/plugins
HASHICORP_PACKER_PLUGIN_SDK_VERSION?=$(shell $(GO) list -m github.com/hashicorp/packer-plugin-sdk | cut -d ' ' -f2)

.PHONY: all build clean test lint fmt check-fmt deps install install-plugin generate install-packer-sdc

all: build

build:
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) .

install: install-plugin

install-plugin: build
	@rm -rf $(HOME)/.config/packer/plugins/github.com/leoarry/proxmox-lxc/*
	packer plugins install --path ${BINARY_NAME} "$(shell echo "${PLUGIN_FQN}" | sed 's/packer-plugin-//')"
	@echo "Plugin installed to new Packer plugin directory"
	@echo "You can now use 'packer plugins installed' to verify"

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -f coverage.txt

test:
	$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-short:
	$(GO) test -v -short ./...

lint:
	$(GOLINT) run ./...

fmt:
	$(GOFMT) -w .

check-fmt:
	@test -z "$$($(GOFMT) -l .)" || (echo "Formatting issues found"; exit 1)

deps:
	$(GO) mod download
	$(GO) mod tidy

install-packer-sdc:
	$(GO) install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@$(HASHICORP_PACKER_PLUGIN_SDK_VERSION)

generate: install-packer-sdc
	$(GO) generate ./...
	rm -rf .docs
	packer-sdc renderdocs -src docs -partials docs-partials/ -dst .docs/
	chmod +x ./.web-docs/scripts/compile-to-webdocs.sh
	./.web-docs/scripts/compile-to-webdocs.sh "." ".docs" ".web-docs" "leoarry"
	rm -r ".docs"

release:
	goreleaser release --clean
