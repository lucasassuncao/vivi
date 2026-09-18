#!/bin/sh
# Populates a dev-mode Vault with enough shape to exercise every corner of vivi:
# both KV versions, a non-KV mount, an empty mount, deep paths, a secret with a
# real version history including a deleted and a destroyed version, values that
# are not strings, a path that is both a secret and a directory, policies, two
# auth methods with roles, and a subtree that a limited token cannot read.
set -eu

say() { printf '\n\033[1;35m==> %s\033[0m\n' "$1"; }

say "Waiting for Vault"
until vault status >/dev/null 2>&1; do sleep 1; done

# ---------------------------------------------------------------- mounts -----

say "Enabling mounts"

# kv/ is versioned: history, soft-delete, destroy, check-and-set.
vault secrets enable -path=kv -version=2 kv 2>/dev/null || echo "  kv/ already enabled"

# legacy/ is unversioned. vivi degrades its interface here: no version list, no
# rollback, and a delete that gets the strong confirmation because v1 has no undo.
vault secrets enable -path=legacy -version=1 kv 2>/dev/null || echo "  legacy/ already enabled"

# A non-KV mount, to show a mount vivi lists but cannot browse.
vault secrets enable pki 2>/dev/null || echo "  pki/ already enabled"

# Enabled and deliberately never written to. An empty mount is a state vivi has
# to render honestly - an open arrow pointing at nothing reads as a failed load -
# and without a mount reserved for it there is nothing to look at.
vault secrets enable -path=empty -version=2 kv 2>/dev/null || echo "  empty/ already enabled"

# ---------------------------------------------------------------- secrets ----

say "Writing secrets"

# Four versions, so the version list has something to show and diff.
vault kv put -mount=kv app/prod/db username=svc_app password=first-password >/dev/null
vault kv put -mount=kv app/prod/db username=svc_app password=second-password >/dev/null
vault kv put -mount=kv app/prod/db username=svc_app password=third-password >/dev/null
vault kv put -mount=kv app/prod/db \
  username=svc_app \
  password=hunter2-current \
  host=db.prod.internal \
  port=5432 >/dev/null

# v2 soft-deleted (undelete brings it back), v1 destroyed (nothing does).
vault kv delete -mount=kv -versions=2 app/prod/db >/dev/null
vault kv destroy -mount=kv -versions=1 app/prod/db >/dev/null

vault kv put -mount=kv app/prod/cache url=redis://cache.prod:6379 tls=true >/dev/null
vault kv put -mount=kv app/prod/queue url=amqp://mq.prod:5672 vhost=/prod >/dev/null

vault kv put -mount=kv app/stage/db username=svc_stage password=stage-password host=db.stage.internal >/dev/null
vault kv put -mount=kv app/stage/cache url=redis://cache.stage:6379 >/dev/null

vault kv put -mount=kv platform/ci/github token=ghp_example_token >/dev/null
vault kv put -mount=kv platform/ci/registry username=ci password=registry-password >/dev/null
vault kv put -mount=kv platform/observability/grafana admin_password=grafana-password >/dev/null

# A multi-line value, to see how the panel and the editor handle one.
vault kv put -mount=kv platform/tls/internal-ca \
  certificate='-----BEGIN CERTIFICATE-----
MIIBexampleexampleexampleexampleexampleexampleexampleexampleexamp
-----END CERTIFICATE-----' >/dev/null

# The subtree the demo token may not read.
vault kv put -mount=kv restricted/root-credentials username=root password=very-secret >/dev/null

# KV v1: no history at all.
vault kv put -mount=legacy old/app key=no-history-here owner=platform >/dev/null
vault kv put -mount=legacy old/api token=legacy-token >/dev/null

# secret/ is created by dev mode itself, not by this script. It is filled with the
# shapes the other mounts do not cover.

# A path that is both a secret and a directory. Vault allows this, and such an
# entry appears twice in one listing: once as a leaf, once with a trailing slash.
vault kv put -mount=secret app owner=platform tier=1 >/dev/null
vault kv put -mount=secret app/database url=postgres://db.internal:5432 >/dev/null
vault kv put -mount=secret app/queue url=amqp://mq.internal:5672 >/dev/null

# Values that are not strings. Vault returns these as JSON numbers, booleans and
# nested objects, and the editor has to round-trip them unflattened.
vault kv put -mount=secret types \
  enabled=true \
  replicas=3 \
  ratio=0.75 \
  limits='{"cpu":"500m","memory":"1Gi"}' \
  regions='["eu-west-1","us-east-1"]' >/dev/null

# More keys than a detail pane can show at once, so the field cursor has to
# scroll to follow itself.
vault kv put -mount=secret wide \
  key01=a key02=b key03=c key04=d key05=e key06=f key07=g key08=h \
  key09=i key10=j key11=k key12=l key13=m key14=n key15=o key16=p \
  key17=q key18=r key19=s key20=t >/dev/null

# ---------------------------------------------------------------- policies ---

say "Writing policies"

vault policy write app-prod-db - >/dev/null <<'EOF'
# Read-only on a single production secret.
path "kv/data/app/prod/db" {
  capabilities = ["read"]
}

path "kv/metadata/app/prod/db" {
  capabilities = ["read", "list"]
}
EOF

vault policy write app-admin - >/dev/null <<'EOF'
# Full control over the application subtree, history included.
path "kv/data/app/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "kv/metadata/app/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "kv/delete/app/*"   { capabilities = ["update"] }
path "kv/undelete/app/*" { capabilities = ["update"] }
path "kv/destroy/app/*"  { capabilities = ["update"] }
EOF

vault policy write platform-readonly - >/dev/null <<'EOF'
path "kv/data/platform/*" {
  capabilities = ["read"]
}

path "kv/metadata/platform/*" {
  capabilities = ["read", "list"]
}
EOF

# The policy behind the demo token: it can see most of the Vault, and is denied
# kv/restricted/* on purpose so vivi has a subtree to render as locked.
vault policy write vivi-demo - >/dev/null <<'EOF'
# Listing the mount table is what fills the tree's first level.
path "sys/mounts" {
  capabilities = ["read"]
}

# The Policies and Auth tabs.
path "sys/policy"          { capabilities = ["read", "list"] }
path "sys/policy/*"        { capabilities = ["read"] }
path "sys/policies/acl"    { capabilities = ["read", "list"] }
path "sys/policies/acl/*"  { capabilities = ["read"] }
path "sys/auth"            { capabilities = ["read", "list"] }
path "auth/approle/role"   { capabilities = ["list"] }
path "auth/approle/role/*" { capabilities = ["read"] }
path "auth/userpass/users"   { capabilities = ["list"] }
path "auth/userpass/users/*" { capabilities = ["read"] }

# Everything under kv/ and legacy/ ...
path "kv/data/*"     { capabilities = ["create", "read", "update", "delete", "list"] }
path "kv/metadata/*" { capabilities = ["create", "read", "update", "delete", "list"] }
path "kv/delete/*"   { capabilities = ["update"] }
path "kv/undelete/*" { capabilities = ["update"] }
path "kv/destroy/*"  { capabilities = ["update"] }
path "legacy/*"      { capabilities = ["create", "read", "update", "delete", "list"] }

# ... and the two dev-mode mounts, or they would render as denied rather than as
# what they are: one full of odd shapes, one deliberately empty.
path "secret/data/*"     { capabilities = ["create", "read", "update", "delete", "list"] }
path "secret/metadata/*" { capabilities = ["create", "read", "update", "delete", "list"] }
path "secret/delete/*"   { capabilities = ["update"] }
path "secret/undelete/*" { capabilities = ["update"] }
path "secret/destroy/*"  { capabilities = ["update"] }
path "empty/data/*"      { capabilities = ["create", "read", "update", "delete", "list"] }
path "empty/metadata/*"  { capabilities = ["create", "read", "update", "delete", "list"] }
path "empty/delete/*"    { capabilities = ["update"] }
path "empty/undelete/*"  { capabilities = ["update"] }
path "empty/destroy/*"   { capabilities = ["update"] }

# ... except this subtree, which vivi shows as denied instead of hiding. The
# exact paths are denied as well as the globs: a LIST arrives as
# "kv/metadata/restricted", with no trailing slash for "restricted/*" to match.
path "kv/data/restricted"       { capabilities = ["deny"] }
path "kv/data/restricted/*"     { capabilities = ["deny"] }
path "kv/metadata/restricted"   { capabilities = ["deny"] }
path "kv/metadata/restricted/*" { capabilities = ["deny"] }
EOF

# -------------------------------------------------------------------- auth ---

say "Enabling auth methods"

vault auth enable approle 2>/dev/null || echo "  approle/ already enabled"
vault write auth/approle/role/db-writer \
  token_policies=app-admin token_ttl=1h token_max_ttl=4h >/dev/null
vault write auth/approle/role/ci \
  token_policies=app-prod-db,platform-readonly token_ttl=20m >/dev/null

vault auth enable userpass 2>/dev/null || echo "  userpass/ already enabled"
vault write auth/userpass/users/lucas \
  password=lucas token_policies=app-admin,platform-readonly >/dev/null
vault write auth/userpass/users/auditor \
  password=auditor token_policies=platform-readonly >/dev/null

# ------------------------------------------------------------------ tokens ---

say "Creating the demo token"
DEMO_TOKEN=$(vault token create -policy=vivi-demo -ttl=8h -display-name=vivi-demo -field=token)

cat <<EOF

  Vault is up at http://127.0.0.1:8200

  Root token (sees everything):
    export VAULT_ADDR=http://127.0.0.1:8200
    export VAULT_TOKEN=root

  Demo token (denied on kv/restricted/, so the tree shows a locked subtree):
    export VAULT_ADDR=http://127.0.0.1:8200
    export VAULT_TOKEN=$DEMO_TOKEN

  PowerShell:
    \$env:VAULT_ADDR = "http://127.0.0.1:8200"
    \$env:VAULT_TOKEN = "$DEMO_TOKEN"

  Then run: vivi

  Things worth opening:
    kv/app/prod/db              4 versions, one deleted, one destroyed
    kv/platform/tls/internal-ca a multi-line value
    legacy/old/app              KV v1: no history, delete has no undo
    kv/restricted/              denied with the demo token
    secret/app                  a path that is both a secret and a directory
    secret/types                numbers, booleans and nested JSON
    secret/wide                 more keys than the pane can show at once
    empty/                      an empty mount, on purpose
    pki/                        a mount vivi lists but cannot browse

EOF
