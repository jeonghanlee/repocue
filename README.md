# RepoCue

RepoCue maintains deterministic Git and filesystem context in a local SQLite
cache and returns compact, freshness-aware JSON cues for coding agents.

## Scope

This repository implements the accepted first vertical slice, Core Evaluation
& Hardening, and the Real Agent A/B Validation adapter: deterministic state,
model-neutral evaluation, Codex JSONL observation, and benchmark scoring.

**Out of scope:** MCP, Tree-sitter, embeddings, LLM integration, filesystem
watching, checkpoints, pruning, and retention-policy enforcement.

## Build

```bash
go build -o repocue ./cmd/repocue
go build -o repocue-codex-runner ./cmd/repocue-codex-runner
```

## CLI

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
