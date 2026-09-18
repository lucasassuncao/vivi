# Plain mkdir as a directory target: on Windows the recipe may run under cmd.exe,
# where `mkdir -p` creates a directory named "-p", and $(wildcard) in an ifeq
# expands at parse time, before `make clean test-coverage` has deleted it.
$(COVERAGE_DIR):
	@mkdir $(COVERAGE_DIR)

.PHONY: test test-watch test-coverage

test: $(GOTESTSUM) ## Run tests with gotestsum (testdox format)
	@$(GOTESTSUM) --format testdox -- -race ./...

test-watch: $(GOTESTSUM) ## Run tests in watch mode (reruns on file changes)
	@$(GOTESTSUM) --format testdox --watch -- -race ./...

# Order-only (after the |): writing a report changes the directory's timestamp.
test-coverage: $(GOTESTSUM) $(GOCOBERTURA) | $(COVERAGE_DIR) ## Run tests with coverage (HTML + Cobertura XML)
	@$(GOTESTSUM) --format testdox -- -race -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./...
	@go tool cover -func=$(COVERAGE_OUT) | tail -1
	@go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@$(GOCOBERTURA) < $(COVERAGE_OUT) > $(COVERAGE_XML)
	@echo "Reports: $(COVERAGE_HTML) | $(COVERAGE_XML)"
