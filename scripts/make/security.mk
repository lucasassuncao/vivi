.PHONY: security govulncheck sbom vuln

security: $(GOSEC) ## Run security analysis with gosec
	@$(GOSEC) -stdout -severity medium ./...

# Unlike a go.sum scan, govulncheck only reports advisories whose vulnerable
# symbol is reachable from this module's code.
govulncheck: $(GOVULNCHECK) ## Report vulnerabilities reachable from this code
	@echo "Checking for reachable vulnerabilities..."
	@$(GOVULNCHECK) ./...

sbom: $(SYFT) ## Generate a CycloneDX SBOM of the dependency tree
	@echo "Generating SBOM..."
	@$(SYFT) . --source-name $(BINARY_NAME) --exclude './.gobin/**' --exclude './$(BUILD_DIR)/**' --exclude './dist/**' -o cyclonedx-json=$(SBOM_FILE)

vuln: $(GRYPE) sbom ## Scan the SBOM for known vulnerabilities with grype
	@echo "Scanning for vulnerabilities..."
	@$(GRYPE) sbom:$(SBOM_FILE) --fail-on medium
