.PHONY: stage-docs serve-docs

# The Pages job in CI runs this too, so the local preview and the published site
# are built from the same layout.
stage-docs: ## Assemble the docs site into ./site (docs/pages, the markdown, images and icons)
	@$(call script,stage-docs) docs $(SITE_DIR)

# docsify fetches the .md files at runtime, which browsers block on file://, so
# the site needs a server.
serve-docs: $(CADDY) stage-docs ## Stage the docs site and serve it on http://localhost:8080 (Ctrl+C to stop)
	@echo "Landing page: http://localhost:8080/"
	@echo "Docs:         http://localhost:8080/docs/"
	@$(CADDY) file-server --root $(SITE_DIR) --listen :8080
