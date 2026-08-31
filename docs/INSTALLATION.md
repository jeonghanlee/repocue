# RepoCue Installation

## Scope

This document covers local builds, user installation, cross-builds, local
documentation generation, and hosted documentation deployment.

**Out of scope:** system-wide installation, package-manager publication, and release signing.

## Prerequisites

| Tool | Required for | Notes |
|---|---|---|
| GNU Make | Makefile workflows | The configuration uses GNU Make features. |
| Go 1.24 or newer | Build and verification | The module version is defined in `go.mod`. |
| `install`, `realpath` | User installation and safe cleanup | GNU coreutils provides these commands on Linux. |
| mdBook | Documentation build | Optional unless `docs.build` is used. |

Check the build tools:

```bash
make check-tools
```

## Build and Verify

Run the normal verification path and build both local binaries:

```bash
make check
make build
```

Generated binaries are written below `build/bin/`. The default build sets `CGO_ENABLED=0`.

The primary CLI can also be built for the supported distribution targets:

```bash
make build.linux-amd64
make build.windows-amd64
```

Cross-build outputs are written below `build/cross/`.

## User Installation

The default destination is `$HOME/.local/bin/repocue`. Installation does not require `sudo`.

Preview, apply, and verify the installation as separate operations:

```bash
make install.dry-run
make install.apply
make install.check
```

If `$HOME/.local/bin` is not in `PATH`, `install.check` reports the mismatch instead of treating the installed file as active.
`install.apply` still completes the file installation and prints a warning so PATH activation and verification remain separate steps.

Activate the default installation path in the current shell, then verify it:

```bash
export PATH="$HOME/.local/bin:$PATH"
make install.check
```

Persist the same PATH entry through the shell configuration used by the local account.

Override the installation prefix without changing tracked files:

```bash
make install.dry-run INSTALL_LOCATION="$HOME/.local/repocue"
make install.apply INSTALL_LOCATION="$HOME/.local/repocue"
export PATH="$HOME/.local/repocue/bin:$PATH"
make install.check INSTALL_LOCATION="$HOME/.local/repocue"
```

Persistent site overrides may be placed in `CONFIG_SITE.local` or `configure/CONFIG_SITE.local`. These files are ignored by Git.

## Removal

Only the executable at the configured destination is removed:

```bash
make uninstall.dry-run
make uninstall.apply
make uninstall.check
```

RepoCue cache data is not removed by the uninstall target.

For a custom installation prefix, pass the same `INSTALL_LOCATION` to every
removal operation:

```bash
make uninstall.dry-run INSTALL_LOCATION="$HOME/.local/repocue"
make uninstall.apply INSTALL_LOCATION="$HOME/.local/repocue"
make uninstall.check INSTALL_LOCATION="$HOME/.local/repocue"
```

## Documentation

Build the local mdBook site under `docs/book/`:

```bash
make docs.build
```

Remove generated documentation output:

```bash
make docs.clean
```

### GitHub Pages

The `.github/workflows/pages.yml` workflow builds the mdBook site in the
`jeonghanlee/mdbook` container and deploys `docs/book/` to GitHub Pages. It runs
when relevant files change on `master` and may also be started manually.

Before the first deployment, configure the repository in GitHub:

1. Open **Settings** and select **Pages**.
2. Under **Build and deployment**, set **Source** to **GitHub Actions**.
3. Run the **Deploy mdBook to GitHub Pages** workflow or push a matching change
   to `master`.

Deployment is complete when the workflow's **Deploy to GitHub Pages** job
passes and reports the site URL.

## Direct Go Workflow

The Makefile is a convenience layer over standard Go commands:

```bash
go test ./...
CGO_ENABLED=0 go build -trimpath -o repocue ./cmd/repocue
```

The resulting `./repocue` executable is separate from the Make-managed
`build/` directory.
