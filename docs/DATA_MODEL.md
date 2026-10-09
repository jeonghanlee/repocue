# RepoCue Data Model

## Scope

This document defines core repository state and quantitative-reporting data.

**Out of scope:** checkpoint tables, pin state, pruning metadata, and retention
policy.

## State Hierarchy

```text
repository
  -> epoch
       -> baseline snapshot
       -> refresh snapshot
            -> semantic delta from its parent
```

Exactly one epoch is active for a repository. A rebaseline changes the active
epoch and keeps the preceding epoch as `superseded`.

## Stable Public IDs

- Repository: `repo-<root-digest>`
- Epoch: `epoch-000001`
- Snapshot: `snapshot-000001`
- Delta: `delta-000001`
- File entity: `file:<repository-relative-path>`
- Calibration: `calibration-<sha256>`

Sequence values are allocated transactionally and stored as explicit text
identifiers. SQLite internal row IDs are not part of the public model.

The CLI may keep a versioned `location.json` beside `state.db`. This disposable
hint contains the verified root, Git directory, and a validation digest, not
snapshot data. It does not alter the SQLite schema or identity rules and is
not included in the SQLite database-size measurement.

## Snapshot Basis

Each snapshot stores branch, HEAD, staged paths, unstaged paths, untracked
paths, dirty state, a Git-state digest, a repository file-state digest, and an
observation time. HEAD and working-tree state remain separate.

The observation time identifies the initial Git basis capture. Repository
files and Git state are assumed unchanged during each scan. This assumption
does not weaken the SQLite transaction or expected-current-snapshot checks.

## File State

Each tracked path records its stable entity ID, Git index mode and object ID,
working-tree mode, existence, byte size, SHA-256 content digest, file type,
language, document classification, and working-tree state. File contents are
never stored.

## Repository-State Semantic Delta

The first slice emits file and repository operations that describe meaningful
changes in the indexed repository state:

- `file.added`
- `file.deleted`
- `file.restored`
- `file.content_changed`
- `file.metadata_changed`
- `file.state_changed`
- `repository.branch_changed`
- `repository.head_changed`
- `repository.working_tree_changed`

Delta items store optional before and after values so a future retention or
checkpoint implementation can reference immutable snapshots without changing
the public state model.

One file path has one operation in a delta. When content and metadata change
together, the operation remains `file.content_changed` and its cue projection
also includes every changed metadata field so the concurrent change is not
lost.

These operations are not code-symbol semantic deltas. Later parsers may add
separate symbol, dependency, and document operations without changing the
existing file-level operation names.

## Measurement

Operation runs store state-preparation duration, scan duration, Git command
count, files scanned, bytes scanned, changed-file count, epoch, and snapshot.
Cue runs store view, token budget, exact output bytes, and estimated output
tokens. Database size is measured from the SQLite file at report time.

Evaluation reports remain versioned JSON artifacts rather than canonical
repository state. They distinguish `observed` from `not_observed` measurements
and keep tokenizer-specific token counts separate from the runtime byte-based
estimate.

Real-agent reports additionally distinguish `derived` and `estimated` values.
They retain runner metadata, Codex token usage, command observations, the final
structured benchmark answer, deterministic Git-fact scores, and projected
reuse totals. These artifacts are not persisted in the repository state
database.

M2 condition reports share a maintenance ID so baseline and refresh cost can
be counted once across conditions and consumers. Each report records the
condition, run index, repository state fingerprint, RepoCue snapshot, cue
schema and size, runner observation, and limitations. A complete five-report
set is staged outside the evaluated repository and published as one directory
only after every condition and repository-state check passes.

M2 does not change the SQLite schema. Structural oracle candidates exist only
while composing an evaluation cue. A future structural index must use stable
entity IDs and content-digest keys, record parser backend and version
provenance, and re-analyze only changed source files. Retention behavior for
such an index belongs to the later storage-lifecycle design.

## Calibration And Reporting

Schema 3 adds `calibration_references`: calibration ID, repository ID, local
epoch and snapshot, relative artifact path, SHA-256 digest, binding observation
time, content fingerprint and algorithm, contract digest, model, and benchmark.
Existing state and measurement rows are preserved. There are no new
measurement-history or structural tables.

Immutable `calibrations/<id>.json` files beside `state.db` use
`repocue/calibration-1`. They retain numeric paired samples, source-report
digests and snapshot IDs, producer/cue/runner contracts, correctness outcomes,
optional qualitative scores and dates, local binding, and limitations. Identity
excludes import time; reimport returns the original observation. Missing or
modified referenced files are errors, not usable profiles.

External `repocue/calibration-evidence-1` names reports, cue stdout, Git events,
scoring basis, and assessments using relative paths and digests. It preserves
the original evaluation fingerprint separately from
`repocue/content-evidence-1`, which combines Git status/index, tracked state,
and sorted non-ignored untracked path/kind/mode/content digests. Absolute
evaluated paths and raw responses are not copied into calibration. Existing
SQLite repository-root metadata remains unchanged.

`repocue/savings-report-1` is stdout-only. Measurements identify classification,
unit, accounting boundary, and unavailable values. Statistics retain pair
values, medians, ranges, counts, and unknown coverage. JSON and Markdown share
one calculation result.

State selection uses the greatest observation strictly before cutoff, with
snapshot sequence resolving ties. Binding selection uses its evidence
observation. Activity uses `[start, cutoff)` recording times; maintenance uses
all baseline/rebaseline and refresh rows in the selected epoch before cutoff,
including pre-window rows. These are not publication-time claims.

Version-1 migration supplied zero defaults for scan duration and Git count.
Without independent provenance these zeros remain `not_observed`, even in a
version-2 DB. Valid zero scanned bytes remain zero. Partial known-only
statistics never replace full totals, trends, maintenance, or break-even.
DB size is a dated current observation; historical size and routine cue-runtime
trends have no retained measurement path.

## Independent task context values

`repocue/task-context-1` is an author-owned tracked declaration, not a SQLite
table. Each item has stable ID/kind, explicit task/path dimensions, required
status, primary source, and supporting check references. A source identity
hashes canonical path, expected full-file digest, and inclusive range.
The declaration digest belongs to its maintained snapshot.

`repocue/task-cue-1` contains repository/epoch/snapshot/basis/freshness,
declaration identity, normalized request, required IDs, whole delivered
items/sources, extracted text digests, and optional omission accounting.
Source text is transient. Existing cue metrics record view `task`.

Benchmark, answer, runner, and evaluation have independent task version 1.
External evaluation values contain frozen task/policy/rate/schema digests, settings,
content fingerprints, full CLI costs, raw-artifact digests, and every attempt.
Attempt journals distinguish started, completed, failed, invalid, and cancelled.
Pair reports bind the actual answer schema with `output_schema_digest`.
Runner requests require that digest; observations retain the captured digest
or null when schema capture is unavailable. Each attempt stores the schema
bytes passed through its captured invocation.

Summary inventory keeps delivery, correctness, compliance, metric status,
eligibility, and errors separately. Per-metric paired coverage and differences
remain separate from a gated verdict. Assessments bind exact task/policy,
attempt/answer, source basis, assessor, and rubric. No task result is a
calibration reference. See [Task contracts](TASK_CONTEXT.md).
