.PHONY: build build-all release tag install run

build: $(GORELEASER) ## Build binary with goreleaser (current platform only)
	@echo "Building..."
	@$(GORELEASER) build --skip=validate --single-target --snapshot --clean

build-all: $(GORELEASER) ## Build binaries for all platforms
	@echo "Building for all platforms..."
	@$(GORELEASER) build --skip=validate --snapshot --clean

release: $(GORELEASER) ## Create a release with goreleaser
	@echo "Creating release..."
	@$(GORELEASER) release --timeout 360s

tag: ## Create and push an annotated git tag (usage: make tag VERSION=v1.2.3)
ifndef VERSION
	$(error Usage: make tag VERSION=v1.2.3)
endif
	git diff --exit-code --quiet
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

install: ## Install binary globally
	@go install

run: ## Run the application
	@go run $(MAIN_PATH) --theme plain
