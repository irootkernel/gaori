GO ?= go
GOLANGCI_LINT ?= golangci-lint
BINARY := gaori
BIN_DIR := bin
VERSION ?= 0.1.15
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
TOOLCHAIN_ROOT ?= $(HOME)/.local/gaori/toolchains
TOOLCHAIN_VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null | sed 's/^v//' || true)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

UNIT_PACKAGES := ./internal/artifacts ./internal/cli ./internal/config ./internal/extract ./internal/insights ./internal/rules ./internal/runner ./internal/safety ./internal/tagset
INTEGRATION_PACKAGES := ./internal/cli
E2E_PACKAGES := ./e2e
GUARDRAIL_TEST_PATTERN := ^(TestAwaitRunDocumentationContract|TestMCPDocumentationAndSkillContract|TestParserSupportDocumentationContract|TestRepositoryTestFunctions|TestRepositoryTestStageClassification|TestRepositoryUsesGaoriIdentity|TestRequirementTraceabilityAuditRejectsInvalidEvidence|TestRequirementTraceabilityMatrixCoversCompletedRequirements|TestUseGaoriCleanupAdvisoryContract|TestUseGaoriStatusSkillContract)$$

.PHONY: build install install-toolchain test test-prepare test-unit test-int test-e2e format lint vet guardrails clean

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/$(BINARY)

install:
	env -u GOPATH $(GO) install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY)

install-toolchain:
	@GAORI_INSTALL_VERSION="$(VERSION)" GAORI_INSTALL_TOOLCHAIN_VERSION="$(TOOLCHAIN_VERSION)" GAORI_INSTALL_ROOT="$(TOOLCHAIN_ROOT)" GAORI_INSTALL_BIN_DIR="$(BIN_DIR)" GAORI_INSTALL_BINARY="$(BINARY)" ./scripts/install-toolchain

test:
	$(MAKE) test-prepare
	$(MAKE) test-unit
	$(MAKE) test-int
	$(MAKE) test-e2e

test-prepare:
	$(MAKE) format
	$(MAKE) lint
	$(MAKE) vet
	$(MAKE) guardrails
	$(MAKE) build
	@echo "[test] test-prepare complete"

format:
	$(GO) fmt ./...

lint:
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --build-tags integration ./internal/cli

vet:
	$(GO) vet ./...
	$(GO) vet -tags=integration ./internal/cli

guardrails:
	$(GO) test -count=1 $(E2E_PACKAGES) -run '$(GUARDRAIL_TEST_PATTERN)'

test-unit:
	$(GO) test -race -count=1 $(UNIT_PACKAGES)
	@echo "[test] test-unit complete"

test-int:
	$(GO) test -race -count=1 -tags=integration $(INTEGRATION_PACKAGES)
	@echo "[test] test-int complete"

test-e2e:
	$(GO) test -count=1 -parallel=1 $(E2E_PACKAGES) -skip '$(GUARDRAIL_TEST_PATTERN)'
	@echo "[test] test-e2e complete"

clean:
	rm -rf $(BIN_DIR)
