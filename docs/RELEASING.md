# Releasing

A release is a git tag. Everything else follows from it.

```bash
make tag VERSION=v0.2.0
```

That target refuses to run with a dirty tree (`git diff --exit-code`), creates an annotated tag and pushes it. The push matches `v*.*.*` in [`.github/workflows/release.yml`](https://github.com/lucasassuncao/vivi/blob/main/.github/workflows/release.yml), which runs GoReleaser, which builds six binaries (linux, darwin and windows, each amd64 and arm64) and attaches them to a GitHub release along with a checksum file.

Run `make all` first. The tag workflow does not re-run the test suite; it trusts that the commit it points at was already green on CI.

## What the build injects

The binary knows two things that are not in the source, both set by `ldflags` in [`.goreleaser.yaml`](https://github.com/lucasassuncao/vivi/blob/main/.goreleaser.yaml):

| Symbol | Becomes |
| --- | --- |
| `internal/cmd.Version` | the tag being built |
| `internal/cmd.DefaultRepo` | `lucasassuncao/vivi`, the repo `self-update` checks |

A binary built any other way (`go build`, `go install`, `make run`) gets neither, and falls back in source: `vivi --version` reports the module version the toolchain stamped in, or `dev` for a build from a working tree, and `--repo` defaults to this repository so `self-update` still works from a `go install`. A build from a working tree deliberately does not claim to be a release.

## The coupling worth knowing about

`self-update` finds its download by **matching the asset filename**. The `name_template` in `.goreleaser.yaml` produces names like `vivi_0.2.0_Windows_x86_64.exe`, and `selectAsset` in [`internal/updater/selfupdate.go`](https://github.com/lucasassuncao/vivi/blob/main/internal/updater/selfupdate.go) scores them against its own `osAliases` and `archAliases` tables: `Windows` and `x86_64` are entries in those tables, not coincidences. The aliases match whole words, so `darwin` is not read as a `win` build.

Change the name template without changing the tables and every already-installed vivi stops finding its upgrade. It fails quietly, as `no compatible binary found in release vX.Y.Z`, and only for users on the old binary, never on your machine, where you build from source.

The asset must also carry positive evidence of its platform. `selectAsset` skips anything that names no OS and is not a `.exe`, because `vivi_amd64` is as likely Linux as Windows and writing the wrong one over the running binary leaves an executable that cannot run.

## Checksums

GoReleaser publishes a checksum file, and `self-update` **fails closed** against it: no manifest, or a manifest it cannot read, means the binary is not installed. `--allow-unverified` is the deliberate override.

This is why a release must never be published with checksums disabled. Doing so does not degrade verification, it breaks self-update outright for anyone who does not pass the flag.

## The token

`release.yml` reads `secrets.GORELEASER_TOKEN`, and that secret has to exist on the repository or the job publishes nothing: an unset secret is an empty string, not a fallback. Publishing here does not strictly need a PAT, since the workflow declares `contents: write` and the `GITHUB_TOKEN` Actions mints per run covers it; one is required only when GoReleaser writes to a *different* repository, such as a Homebrew tap.

One caveat if you ever switch to `GITHUB_TOKEN`: a release created with it does not trigger other workflows. Nothing here listens on `release: published`, so today it costs nothing.

## First release

`self-update` can only be exercised from one release to the next, so the upgrade path is untested until a second tag exists. Plan on cutting a patch release soon after the first one, purely to walk it.
