# RepoCue Core Architecture

## Scope

This document defines the component boundaries and data flow for the accepted
Go core, the M2 structural-context experiment, and quantitative reporting.

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
  -> capture Git basis once (repository remains unchanged during the scan)
  -> scan all tracked files or changed tracked files
  -> commit epoch, snapshot, file state, delta, and run metrics in one SQLite transaction
  -> project deterministic JSON
```

Repository content is hashed as a stream while retaining at most the first
8 KiB for binary classification; file size does not determine scanner memory.
Content is not stored. Ignored files are excluded.
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
- SQLite schema version 3 adds calibration references. Versions 1 and 2
  migrate transactionally; read-only reports accept versions 2 and 3.

## Transaction Boundary

Repository scanning happens before the SQLite write transaction. Files and Git
state must remain unchanged for the duration of each scan. The scanner captures
one Git basis and records its initial observation time; it does not recapture
the basis after scanning or revisit every scanned path. Regular-file reads
retain their local before/after metadata check, but this is not a guarantee
against repository-wide concurrent mutation.

The database transaction verifies the expected current snapshot before
publishing the new snapshot. An interrupted write therefore cannot leave a
partially-current snapshot. Changes between scans are still detected by a new
Git basis and changed-file scan. After a concurrent filesystem edit, rerun
refresh when the repository is idle.

Each full or incremental scan normally uses three Git commands: porcelain v2
status supplies HEAD and changed paths; `symbolic-ref --quiet --short HEAD`
supplies the branch; one tagged index listing supplies file entries, the index
digest, visibility flags, and conflict checks.
The index digest preserves the untagged `ls-files --stage -z` byte format, so
existing snapshots remain comparable. Branch identity preserves Git's short
symbolic-ref format, including `heads/main` when a `main` tag creates ambiguity.
A branch literally named `(detached)` remains distinct from detached HEAD.
Status disables optional index writes.

CLI repository discovery normally adds one call. At an ordinary repository
root, a valid location hint removes that call: `status`, `refresh`, or an
overview `cue` then uses three calls instead of four. A gitlink requires two
additional diff queries to preserve configuration-sensitive submodule behavior.
A repository or Git-directory path
containing a newline requires two additional discovery queries because
`rev-parse` does not delimit its path output with NUL. Counts cover Git processes
invoked by RepoCue, not additional children started internally by Git. Ranked
cues and evaluation may perform additional context and state checks.

## CLI Location Cache

The CLI stores a disposable `location.json` next to `state.db`, under the same
cache-root selection rules. It is written only after Git confirms that the
requested path is the ordinary repository root with a real `.git` directory.
Reuse checks the canonical root, Git-directory metadata, local configuration
and HEAD contents, and Git executable path/metadata. It never reuses a branch,
HEAD value, dirty-state answer, or freshness result; scans still query live Git.

Missing, malformed, oversized, or mismatched hints cause a new Git discovery.
Moves produce a different root identity; changed local configuration or Git
layout invalidates reuse. Subdirectory requests, linked worktrees, separate Git
directories, configuration includes/extensions, explicit worktree configuration,
and Git environment overrides retain Git discovery. Trace and pager variables
do not disable reuse. Eligibility is conservative rather than a second Git
configuration parser. The direct repository API and evaluation harness remain
uncached.

Hint replacement uses a private temporary file and rename. Read failures are
cache misses; write failures produce a warning on stderr while the successful
Git discovery remains usable. Warnings are buffered until command completion;
if the command fails, its JSON error precedes any warning. The hint is not
canonical state, does not change the SQLite schema, and may be removed without
losing snapshots. Its file reads and discovery cost are outside the existing
scan-only metrics.

## Evaluation Boundary

The evaluation harness creates its SQLite state in a temporary directory and
does not modify the evaluated repository. A repository-only probe measures the
full baseline, no-op update, cue projection, content bytes read, file count,
Git command count, database size, and wall duration.

Optional direct and assisted runners are separate executables. Runner-specific
tokenizers and agent event formats remain outside the core packages. The
harness verifies that each runner leaves the repository state unchanged.

Facts derived from live Git commands are accepted only when scans before and
after the read match the persisted snapshot. Ranked cues fail instead of
combining an older snapshot envelope with facts from a newer repository state.

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

M2 output and temporary paths must be outside the evaluated worktree and Git
directory. The harness validates all five conditions before publishing a report-set directory
with one atomic rename. A failed condition therefore exposes no part of its
five-report set and the same run can be retried.

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
The harness rejects negative or internally inconsistent metrics, invalid
measurement status values, mismatched repository or benchmark metadata, and
condition sets that do not share identical runner metadata.

## Deployment Constraint

The default build remains CGo-free. Linux static linking and Windows amd64
cross-building are release constraints. A future structural backend must be
measured against those constraints before it becomes a production dependency.

## Quantitative Reporting Boundary

`internal/evidence` defines external calibration provenance and contained,
digest-checked file references. `repository.CaptureEvidence` adds streamed
non-ignored untracked content to Git and tracked-file evidence without changing
core refresh scope. Unreadable entries prevent a complete capture; symlinks
contribute target text, not target contents.

Explicit `evaluate-m2 --conditions direct,basic` measures the shipped cue
executable and supplies its captured stdout to the assisted runner. It records
wall duration, cache/hint state, executable digest, and digest-linked Git
Trace2 process sessions. Builder duration is a separate diagnostic. This path
needs no oracle; the default five-condition experiment remains unchanged.

`internal/reporting` validates complete compatible external pairs, recomputes
the five deterministic Git facts, optionally joins qualitative assessments,
and publishes compact numeric calibration. Raw answers, prompts, scoring
basis, and assessment narratives remain external. SQLite serializes
publication and rechecks the expected local snapshot. A failed transaction can
leave an unreferenced artifact, never a trusted partial reference.

Reports read state, history, and references in one read-only transaction, then
validate live content. They never refresh, migrate, write cue metrics, invoke
an agent, or store stdout. Source snapshot IDs are provenance, not local DB
identity. Exact content matches are measured references; compatible later
snapshots are estimates. Epoch/contract or same-snapshot untracked-content
mismatches suppress projections. Quality remains dated calibration evidence.

Reuse arithmetic is pairwise and unit-specific. Historical state uses
observation time; activity and full-epoch maintenance use metric recording
time. Neither proves SQLite publication time. Missing measurements and
ambiguous legacy zeros cannot become observed savings. See
[ADR 0002](adr/0002-quantitative-reporting.md) and
[Evaluation](EVALUATION.md#quantitative-reporting).
