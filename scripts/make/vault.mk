.PHONY: test-integration vault-up vault-down vault-logs

# The throwaway Vault used by `make test-integration`.
VAULT_ADDR ?= http://127.0.0.1:8200

# -f because the stack lives under scripts/dev/; -p because without it the
# project would be named "dev" after that directory, and `down` would no longer
# match the containers labelled "vivi".
COMPOSE := docker compose -f scripts/dev/docker-compose.yml -p vivi

test-integration: export VIVI_INTEGRATION := 1
test-integration: export VAULT_ADDR := $(VAULT_ADDR)
test-integration: export VAULT_TOKEN := root
test-integration: $(GOTESTSUM) ## Run tests against the docker-compose Vault (make vault-up first)
	@$(GOTESTSUM) --format testdox -- -count=1 ./internal/vault/

# --build because the seeder is an image now rather than a bind mount, and
# without it an edit to seed.sh would silently keep running the old copy.
vault-up: ## Start a throwaway Vault, seeded with mounts, secrets and policies
	@$(COMPOSE) up -d --wait --build
	@$(COMPOSE) logs seed

vault-down: ## Stop the throwaway Vault and discard its data
	@$(COMPOSE) down -v

vault-logs: ## Show what the seeder created, including the demo token
	@$(COMPOSE) logs seed
