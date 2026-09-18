# Development

Everything needed to work on vivi: the toolchain, the suites, and the throwaway Vault the integration tests run against.

## Build and test

```bash
make tools              # install every pinned tool into ./.gobin
make build              # goreleaser, current platform
make test               # unit suite, in memory, no server
make test-coverage      # the same, with HTML and Cobertura reports
make lint security      # golangci-lint and gosec
make all                # everything above, plus the supply-chain scans
```

Targets live in `scripts/make/*.mk`, one file per area; the Makefile holds the shared variables and the includes. Tools are pinned there and installed into `./.gobin`, never taken from `PATH`, so a target runs the version this repository chose.

The whole unit suite runs against `internal/vault/fake`, an in-memory Vault behind the same port as the real adapter. Nothing below is needed to run `make test`; it is needed to check the request shapes the fake cannot verify.

See [ARCHITECTURE.md](ARCHITECTURE.md) for how the packages are arranged, and [RELEASING.md](RELEASING.md) for cutting a release.

## The development Vault

`make vault-up` starts a Vault in dev mode and seeds it. Dev mode keeps everything in memory, so stopping the container throws the data away, which is the point. Never point it at anything real.

```bash
make vault-up       # start and seed; prints the tokens
make vault-logs     # print them again
make vault-down     # stop it and discard the data
```

The stack is [`scripts/dev/docker-compose.yml`](https://github.com/lucasassuncao/vivi/blob/main/scripts/dev/docker-compose.yml) and the fixture is [`scripts/dev/seed.sh`](https://github.com/lucasassuncao/vivi/blob/main/scripts/dev/seed.sh). The seeder runs as a built image rather than a bind mount, so the same compose file works from the host and from inside the devcontainer, where the daemon is the host's and cannot see `/workspaces`.

### Tokens

| Token | Policy | Sees |
| --- | --- | --- |
| `root` | none, it is the root token | everything |
| printed by the seeder | `vivi-demo` | everything except `kv/restricted/`, with an 8h TTL |

The demo token is the way to watch the tree render a genuinely denied subtree by hand; a root token never produces one. Its value changes on every `make vault-up`, so the integration suite does not depend on it: the tests that need a denial mint their own token and policy under the root token and remove both when they finish.

### Mounts

| Mount | Type | There to exercise |
| --- | --- | --- |
| `kv/` | KV v2 | versions, soft delete, destroy, check-and-set |
| `legacy/` | KV v1 | no history, and a delete with no undo |
| `secret/` | KV v2 | a path that is both a secret and a directory |
| `empty/` | KV v2 | an empty mount |
| `pki/` | PKI | a mount vivi lists but cannot browse |

### Paths worth opening

| Path | Why |
| --- | --- |
| `kv/app/prod/db` | four versions, one deleted, one destroyed |
| `kv/platform/tls/internal-ca` | a multi-line value |
| `kv/restricted/` | denied with the demo token |
| `legacy/old/app` | KV v1: no history, delete has no undo |
| `secret/app` | both a secret and a directory |
| `secret/types` | numbers, booleans and nested JSON |
| `secret/wide` | more keys than the pane can show at once |

Policies: `app-prod-db`, `app-admin`, `platform-readonly`, `vivi-demo`. Auth methods: `approle` (roles `db-writer` and `ci`) and `userpass`, so the auth tab has something to list.

### The integration suite

```bash
make vault-up
make test-integration
```

It is gated twice. `VIVI_INTEGRATION` must be set (`make test-integration` sets it, a plain `make test` does not) and `VAULT_ADDR` must resolve to loopback, whatever the gate says. The suite creates and deletes real paths, so the address guard is not a convenience check and is worth keeping over the annoyance of a remote Vault you cannot point it at.

The tests that need a denied path or a token that expires mint their own token and policy under the root token, and remove both when they finish. Nothing beyond the two gates above is required.

### In the devcontainer

The daemon is the host's (docker-outside-of-docker) so `make vault-up` inside the container starts the Vault as a sibling, not a nested container. It outlives a rebuild, and the host still reaches it on 8200.

`VAULT_ADDR` stays `http://127.0.0.1:8200` in there rather than `host.docker.internal`, because of the loopback guard above. `.devcontainer/vault-forward.sh` makes the name true with socat at startup.

## The documentation site

[lucasassuncao.github.io/vivi](https://lucasassuncao.github.io/vivi/) is a static landing page plus a [docsify](https://docsify.js.org) app that renders the very markdown in this directory. Nothing is generated: the site is assembled by copying.

```plain
docs/
  *.md              the pages, which GitHub also renders here
  _sidebar.md       the docsify navigation
  images/           the recordings and the logo, drawn by EXAMPLES.md and the repository README
  pages/            the site root, uploaded to GitHub Pages
    index.html      the landing page
    style.css
    image.png       the logo, which the static page needs at a fixed path
    images/         copied here by CI, never committed
    docs/
      index.html    the docsify app
      custom.css
```

The `pages` job in CI assembles it before uploading: the markdown, `_sidebar.md` and `images/` all go to `docs/pages/`, the site root docsify reads from. Nothing under `docs/` is stored twice in the repository. The logo is the exception, and it is one because the landing page is static HTML that has to find it whether or not the copy has run: it keeps its own copy in `docs/pages/`, and the repository README reaches down to it.

### Images in the docs

Write them as markdown, `![alt](images/x.gif)`, with no `../` in front, and they render in an editor's preview and on the site both. It is worth knowing why, because the two obvious spellings are the ones that fail.

docsify rewrites a markdown image against `basePath`, the directory it reads the markdown from: `docs/` when the app is served from a checkout, the site root on Pages. A path here therefore means `docs/images/` in the repository and `images/` at the site root, and the copy above puts the directory in both. A `../` in front breaks that symmetry, and an `<img>` tag escapes the rewrite entirely: raw HTML is left as written, so the browser resolves it against the URL the app is served from, which is `/docs/` and never where the markdown lives. Size a markdown image with docsify's own syntax instead, `![alt](images/x.png ':size=313')`.

The landing page is ordinary static HTML sitting at the site root, which is `docs/pages/` in the repository. Its own assets, the logo, live beside it and are committed, so it renders the same opened from disk as it does deployed.

docsify fetches its markdown at runtime, which a browser will not do for a `file://` page: opening `docs/pages/docs/index.html` from the file manager renders **404 - Not found** however correct the files are. The site is read where it is published, or from a local HTTP server with the same copy CI makes.

A page added to `docs/` needs a line in `_sidebar.md` to appear in the navigation. A link from one page to another is relative (`KEYS.md`), and a link to anything outside `docs/` has to be an absolute GitHub URL: the site root is `docs/pages/`, so `../` climbs out of it and 404s.

The site deploys from `main` after the `Checks` job passes, never from a pull request.
