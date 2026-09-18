.PHONY: fmt lint deps clean-buildcache clean-modcache clean-testcache

deps: ## Download and tidy dependencies
	@go mod download
	@go mod tidy

fmt: ## Format code
	@go fmt ./...

lint: $(GOLANGCI) ## Run linter checks
	@$(GOLANGCI) -v run ./...

clean-modcache: ## Clean Go module cache
	@go clean -modcache

clean-testcache: ## Clean Go test cache
	@go clean -testcache

clean-buildcache: ## Clean Go build cache
	@go clean -cache
