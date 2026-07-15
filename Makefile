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

.PHONY: all build clean test test-short test-integration test-integration-full vet-integration lint fmt check-fmt deps install install-plugin generate install-packer-sdc

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

# Runs the integration test suite (build tag "integration"). Tests skip
# themselves at runtime unless PROXMOX_HOST and friends are set, so this
# is safe to run without real Proxmox infra - it still catches compile
# errors in integration_test.go that `make test` never builds.
test-integration:
	$(GO) test -tags integration -v ./... -run TestIntegration

# Compile-only check for the integration-tagged test files, without
# needing real Proxmox infra. Cheap enough to run on every CI build.
vet-integration:
	$(GO) vet -tags integration ./...

# Runs the integration suite against a REAL Proxmox host: creates,
# provisions, and (depending on the test) backs up or templates an actual
# container there. Pass connection details as variables on the command
# line, e.g.:
#
#   make test-integration-full \
#     PROXMOX_HOST=10.0.0.5 PROXMOX_USER=root@pam PROXMOX_PASSWORD=secret \
#     PROXMOX_TEMPLATE=local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst
#
# See builder/lxc/integration_test.go for the full list of supported
# PROXMOX_* variables (storage, ctid, vlan, network_ip, gateway, pigz...).
# RUN narrows which integration tests run (default: all of them).
# TIMEOUT bounds the whole run (default 30m; real builds can be slow).
RUN?=TestIntegration
TIMEOUT?=30m
test-integration-full:
	PROXMOX_HOST=$(PROXMOX_HOST) \
	PROXMOX_PORT=$(PROXMOX_PORT) \
	PROXMOX_USER=$(PROXMOX_USER) \
	PROXMOX_PASSWORD=$(PROXMOX_PASSWORD) \
	PROXMOX_KEY_PATH=$(PROXMOX_KEY_PATH) \
	PROXMOX_TEMPLATE=$(PROXMOX_TEMPLATE) \
	PROXMOX_STORAGE=$(PROXMOX_STORAGE) \
	PROXMOX_CTID=$(PROXMOX_CTID) \
	PROXMOX_BRIDGE=$(PROXMOX_BRIDGE) \
	PROXMOX_VLAN=$(PROXMOX_VLAN) \
	PROXMOX_NETWORK_IP=$(PROXMOX_NETWORK_IP) \
	PROXMOX_GATEWAY=$(PROXMOX_GATEWAY) \
	PROXMOX_BACKUP_PIGZ=$(PROXMOX_BACKUP_PIGZ) \
	$(GO) test -tags integration -v ./... -run '$(RUN)' -timeout $(TIMEOUT)

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
