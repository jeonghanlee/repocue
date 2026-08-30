# RepoCue Core Architecture

## Scope

This document defines the component boundaries and data flow for the accepted
Go core and the M2 structural-context experiment.

**Out of scope:** MCP, production source parsing, semantic embeddings, LLM
processing, filesystem watching, checkpoints, pruning, and retention-policy
enforcement.

## Components

`cmd/repocue` owns process startup. `internal/app` parses CLI requests and
coordinates the concrete repository, storage, snapshot, cue, and metrics
packages. The repository package runs Git and reads tracked working-tree files.
The storage package owns SQLite schema and transactions. The snapshot package
computes repository-state semantic file changes. The cue package selects and
serializes agent-facing JSON within a token estimate. The metrics package
contains measurement rules shared by commands. The evaluation package measures
the shipped repository, storage, and cue paths and optionally invokes external
agent runners through a versioned JSON contract.

SQLite driver selection is isolated in `internal/storage`. The first slice
uses `modernc.org/sqlite` through `database/sql` because it is CGo-free and can
be distributed in the RepoCue executable.

## Data Flow

```text
CLI request
  -> discover Git repository
  -> capture Git basis
  -> scan all tracked files or changed tracked files
  -> verify the Git index, Git basis, and scanned filesystem observations
     remained coherent during the scan
  -> commit epoch, snapshot, file state, delta, and run metrics in one SQLite transaction
  -> project deterministic JSON
```

Repository content is hashed but not stored. Ignored files are excluded.
Non-ignored untracked paths affect dirty-state metadata but their content is
not indexed in this slice.

Git index entries marked `assume-unchanged` or `skip-worktree` are rejected in
this slice because Git may hide working-tree changes for those paths and
RepoCue could not claim current freshness.

## Bootstrap Decisions

- Repository identity uses the first 96 bits of the resolved repository root's
  SHA-256 digest. Moving a checkout creates a distinct local cache.
- Public epoch, snapshot, delta, and file identifiers are text values and do
  not expose SQLite row IDs.
- A snapshot is created only when indexed Git or file state changes. A no-op
  refresh records metrics and retains the existing snapshot.
- Manual rebaseline creates a new active epoch and marks the former epoch
  superseded without deleting its data.
- Semantic delta operations are file-level until a parser is introduced.
- File operations are repository-state semantic deltas. They do not claim
  code-symbol, dependency, or document semantics.
- Freshness is `current` for a clean indexed state, `dirty-but-indexed` when
  the indexed tracked state matches a dirty working tree, and `refresh-needed`
  when the live state differs from the current snapshot.
- Token estimation is `ceil(serialized UTF-8 bytes / 4)`. It is explicitly an
  estimate and is stored with the exact output byte count.
- SQLite schema version 2 records scan duration and Git command count. Schema
  version 1 migrates through an explicit transaction.

## Transaction Boundary

Repository scanning happens before the SQLite write transaction. RepoCue
captures the Git index and basis around a scan, validates scanned filesystem
observations at a defined observation point, and rejects the result when those
observations are not coherent. The database transaction then verifies the
expected current snapshot before publishing the new snapshot. An interrupted
write therefore cannot leave a partially-current snapshot.

## Evaluation Boundary

The evaluation harness creates its SQLite state in a temporary directory and
does not modify the evaluated repository. A repository-only probe measures the
full baseline, no-op update, cue projection, content bytes read, file count,
Git command count, database size, and wall duration.

Optional direct and assisted runners are separate executables. Runner-specific
tokenizers and agent event formats remain outside the core packages. The
harness verifies that each runner leaves the repository state unchanged.

`repocue-codex-runner` is the first adapter. It invokes a fresh ephemeral Codex
non-interactive run, consumes the JSONL event stream, classifies observable
commands, and returns one versioned model-neutral runner observation. The
benchmark package scores deterministic Git facts after the harness verifies the
repository basis. Semantic answers remain available for a separate qualitative
assessment.

## M2 Experiment Boundary

M2 compares five fixed conditions rather than introducing a general extension
framework:

```text
direct
placebo
basic
ranked
structural-oracle
```

One baseline and one no-op refresh prepare a shared maintained state. Each
condition receives a fresh external runner process. All assisted conditions
use the same instruction and context budget. The direct condition receives no
RepoCue context.

The structural oracle is an evaluation tool, not a core backend. A Bash
executable extracts conservative symbol candidates from tracked Bash and
Python files. Go parses its tab-separated records, serializes JSON safely, and
composes the structural condition within the same token estimate as the ranked
condition. M2 adds no parser module, structural database table, AST, call
graph, or persistent structural index.

The runner contract records every observed Codex `turn.completed.usage` event.
Command classification and the sum of current sizes for explicitly named
repository paths are derived measurements. That byte value is named
`named_file_size_proxy_bytes`; it is not observed filesystem I/O.

## Deployment Constraint

The default build remains CGo-free. Linux static linking and Windows amd64
cross-building are release constraints. A future structural backend must be
measured against those constraints before it becomes a production dependency.
