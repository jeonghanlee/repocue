# RepoCue CLI Reference

## Scope

This document covers the current RepoCue CLI commands and their Makefile build wrappers.

**Out of scope:** MCP transport, production structural parsing, checkpoints, pruning, and filesystem watching.

The examples below assume that an installed `repocue` executable is available
in `PATH`. From a repository checkout, run `make build.repocue` and replace
`repocue` with `build/bin/repocue`. A direct Go build described in
[INSTALLATION.md](INSTALLATION.md) instead produces `./repocue`.

## Help

List every command with a one-line summary:

```bash
repocue help
```

Show the synopsis, summary, and flag defaults of one command:

```bash
repocue help cue
repocue cue --help
```

Help output is plain text on stdout with exit code 0. Running `repocue` without a command prints the same usage on stderr with exit code 2. Every other failure exits with code 1 and writes a single-line JSON error as the first line of stderr; an unknown command additionally prints plain-text usage guidance after that line.

---

## Repository State

Initialize the current Git repository with a deterministic full baseline:

```bash
repocue init .
```

Inspect the live Git basis and cached RepoCue state:

```bash
repocue status
```

Refresh changed tracked files and publish a new snapshot only when indexed state changed:

```bash
repocue refresh
```

Start a new epoch with a full baseline while retaining the superseded epoch:

```bash
repocue rebaseline --label milestone:first-prototype --reason manual
```

All state commands accept `--repository PATH` where applicable. Use `--cache-dir PATH` to override the normal cache location.

---

## Cues

Generate a compact overview within an estimated token budget:

```bash
repocue cue --view overview --max-tokens 500
```

Generate a file-level repository-state delta from an earlier snapshot:

```bash
repocue cue --since snapshot-000001 --max-tokens 500
```

Experimental evaluation views remain available for measured comparisons:

```bash
repocue cue --view ranked --max-tokens 500
repocue cue --view provenance --path internal/ --max-tokens 500
```

---

## Metrics and Evaluation

Read recorded baseline, refresh, and cue measurements:

```bash
repocue metrics
```

Run the model-neutral repository evaluation without external agent runners:

```bash
repocue evaluate --repository . --max-tokens 500
```

External runner and M2 experiment contracts are documented in [EVALUATION.md](EVALUATION.md). The `repocue-codex-runner` binary is an evaluation adapter and is not installed by the user installation target.

---

## Makefile Wrappers

Build and verify the CLI:

```bash
make check
make build.repocue
```

Preview and apply user installation:

```bash
make install.dry-run
make install.apply
make install.check
make uninstall.dry-run
make uninstall.apply
make uninstall.check
```
