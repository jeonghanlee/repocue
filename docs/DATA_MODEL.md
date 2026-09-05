# RepoCue First Vertical Slice Data Model

## Scope

This document defines the persisted state needed by the first vertical slice.

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

Sequence values are allocated transactionally and stored as explicit text
identifiers. SQLite internal row IDs are not part of the public model.

## Snapshot Basis

Each snapshot stores branch, HEAD, staged paths, unstaged paths, untracked
paths, dirty state, a Git-state digest, a repository file-state digest, and an
observation time. HEAD and working-tree state remain separate.

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
