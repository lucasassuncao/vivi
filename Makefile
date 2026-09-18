# Targets live in scripts/make/*.mk, one file per area. Variables shared by
# those files are defined here, above the includes: `:=` expands as the line is
# read, so EXE and GOBIN_DIR must already exist when tools.mk builds its paths.
#
# Include order matters for the same reason. tools.mk comes first because the
# others name $(GOLANGCI), $(GOTESTSUM) and friends as prerequisites, and a
# prerequisite is expanded when its rule is read.

# The includes are read before any target of this file, so the first target make
# sees is one of tools.mk's. Without this, that becomes the default goal.
.DEFAULT_GOAL := help

ifeq ($(OS),Windows_NT)
EXE := .exe
else
EXE :=
endif

# Project variables
BINARY_NAME := vivi
BUILD_DIR   := bin
MAIN_PATH   := main.go
SBOM_FILE   := sbom.json

# Where the pinned tools land; tools.mk explains why they are not taken from PATH.
GOBIN_DIR := $(CURDIR)/.gobin

# Coverage
COVERAGE_DIR  := coverage
COVERAGE_OUT  := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html
COVERAGE_XML  := $(COVERAGE_DIR)/coverage.xml

MAKE_DIR := scripts/make

include $(MAKE_DIR)/tools.mk
include $(MAKE_DIR)/go.mk
include $(MAKE_DIR)/build.mk
include $(MAKE_DIR)/test.mk
include $(MAKE_DIR)/security.mk
include $(MAKE_DIR)/vault.mk

.PHONY: help all clean-all

# make appends every included file to MAKEFILE_LIST, so help still sees every
# target in every .mk. Two scripts because neither shell is guaranteed to exist
# on the other's platform.
help: ## Show this help message
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -ExecutionPolicy Bypass -File $(MAKE_DIR)/help.ps1 $(MAKEFILE_LIST)
else
	@sh $(MAKE_DIR)/help.sh $(MAKEFILE_LIST)
endif

# Local, deterministic checks first, so a failure points at the code. The
# supply-chain scans go last because they are the only steps that need the
# network: govulncheck and grype both fetch a vulnerability database, and a
# hiccup there should not mask a lint or test failure. govulncheck runs before
# grype so the reachability answer is printed even when grype fails the build
# on an advisory this binary never executes.
all: deps fmt lint security test-coverage govulncheck sbom vuln

clean-all: clean-tools clean-buildcache clean-testcache clean-modcache ## Remove build artifacts, installed tools and cache
	@echo "Removing $(BUILD_DIR)..."
	@rm -rf $(BUILD_DIR)
	@echo "Removing dist..."
	@rm -rf dist/
	@echo "Removing $(COVERAGE_DIR)..."
	@rm -rf $(COVERAGE_DIR)
	@echo "Removing $(SBOM_FILE)..."
	@rm -rf $(SBOM_FILE)
