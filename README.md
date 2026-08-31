# RepoCue

RepoCue maintains deterministic Git and filesystem context in a local SQLite
cache and returns compact, freshness-aware JSON cues for coding agents.

- [Quick start](docs/QUICK_START.md)
- [Documentation index](docs/README.md)
- [Installation](docs/INSTALLATION.md)
- [CLI reference](docs/CLI.md)
- [Agent skill](skills/repocue/SKILL.md)
- [Architecture](docs/ARCHITECTURE.md)

## Scope

This repository implements the accepted first vertical slice, Core Evaluation
& Hardening, and the Real Agent A/B Validation adapter: deterministic state,
model-neutral evaluation, Codex JSONL observation, and benchmark scoring.

**Out of scope:** MCP, Tree-sitter, embeddings, LLM integration, filesystem
watching, checkpoints, pruning, and retention-policy enforcement.

## Prerequisites

RepoCue requires Go 1.24 or newer. GNU Make provides the standard local build
and installation workflows. mdBook is optional unless the documentation site
is built.

## Makefile Workflow

Verify the repository and build both local binaries:

```bash
make check
make build
```

Preview, apply, and verify a user installation under `$HOME/.local/bin`:

```bash
make install.dry-run
make install.apply
make install.check
```

If `install.check` reports a PATH mismatch, follow the activation steps in
[docs/INSTALLATION.md](docs/INSTALLATION.md).

Build the local documentation site under `docs/book/`:

```bash
make docs.build
```

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for cross-builds, local
overrides, and removal.

## Direct CLI Workflow

Build the primary CLI without Make:

```bash
CGO_ENABLED=0 go build -trimpath -o repocue ./cmd/repocue
```

This keeps the direct Go output separate from the Make-managed `build/`
directory.

Initialize and inspect a repository:

```bash
./repocue init .
./repocue status
./repocue cue --view overview --max-tokens 500
./repocue refresh
./repocue cue --since snapshot-000001 --max-tokens 500
./repocue rebaseline --label milestone:first-prototype
./repocue metrics
./repocue evaluate --repository . --max-tokens 500
```

Agent comparison uses optional external runners. Both arms receive the same
task file, while only the assisted arm receives the generated cue:

```bash
./repocue evaluate --repository . --task-file task.txt --direct-runner ./runner --assisted-runner ./runner
```

The runner protocol and measurement definitions are documented in
`docs/EVALUATION.md`.

The first real-agent benchmark uses
`docs/benchmarks/repository-state-v1.md` and
`docs/schema/benchmark-answer-v1.schema.json`. The Codex adapter requires an
explicit model, reasoning effort, and output schema path so every A/B pair
records reproducible agent conditions.

RepoCue stores state below `$XDG_CACHE_HOME/repocue` when `XDG_CACHE_HOME` is
set, otherwise below `~/.cache/repocue`. Set `REPOCUE_CACHE_DIR` to override
the cache root.

## Verification

```bash
go test ./...
go test -race ./...
go vet ./...
```
