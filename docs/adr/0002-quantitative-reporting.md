# ADR: Quantitative Reporting From Maintained State

## Scope

This decision defines the calibration and reporting boundaries in
[the canonical reporting plan](../milestone-10a4f02.md).
It does not authorize real-agent experiments or establish measured savings.

Date: 2026-09-07
Status: accepted
Source Decisions: D1-D23 in docs/milestone-10a4f02.md

## Context

Runtime maintenance metrics describe RepoCue work, not agent savings. An
agent comparison requires separately produced, compatible direct/basic pairs
and explicit repository, producer, cue, runner, and measurement provenance.
Routine reports must not invoke an agent or silently renew that evidence.

## Decision

Keep imported numeric calibration in immutable versioned JSON below the cache.
SQLite schema 3 stores only its digest, path, identifier, local binding, and
applicability metadata. A transaction verifies the expected current snapshot
before publishing its reference. Read-only reports support schema 2 and 3 and
never migrate, refresh, generate a cue, or persist their output.

Bind source evidence to the target checkout and local epoch/snapshot, not to
coincidentally equal snapshot IDs in independent databases. Versioned evidence
covers Git state, tracked state, and non-ignored untracked content, including
symlink target text without following links. Incomplete evidence cannot bind.
Core snapshots continue indexing tracked files only. Keep detailed source
inventories, raw answers, prompts, absolute evaluated paths, and scoring
expected/actual values outside calibration and SQLite.

An exact verified binding is a measured reference. A compatible different
snapshot in the same epoch permits estimates with profile drift shown. Changed
untracked content under the same snapshot, incompatible contracts, or a new
epoch suppress projections. Quality scores remain dated source observations,
not renewed assessments of subsequent snapshots.

Calculate differences and reuse costs per matched pair before taking medians
and ranges. Future scenarios are estimates, never observed consumer use.
Token subsets are not added twice. Execution duration includes full cue CLI
cost and recorded epoch maintenance, with their measurement boundaries stated.
Agent Git-command classifications, actual Git subprocesses, scan bytes, and
file-size proxies are different units. Missing compatible observations suppress
only the affected total. There is no cross-unit cost score or inferred I/O
savings. Zero/unfavorable per-consumer differences have no finite break-even.

Historical state selection uses parsed observation times strictly before the
cutoff, with snapshot sequence resolving ties. Activity and maintenance use
metric-row recording times. Neither boundary proves SQLite publication time.
Ambiguous legacy scan-duration/Git-count zeros remain unobserved; retain raw
rows and report known/unknown coverage per dimension. Current DB size and live
validation are current observations, not historical facts.

## Consequences

Reports can operate without a model or tokenizer dependency and preserve the
CGo-free distribution. Calibration acquisition remains an explicit external
cost. Missing evidence reduces available projections instead of creating
optimistic zeros. Raw evidence must be retained separately to reproduce or
renew an import. Cache removal also removes its calibration copies.

## Alternatives Considered

Routine agent execution would mix reporting with experiment cost. Persisting
raw responses in SQLite would widen privacy exposure and storage ownership.
Names-only evidence would miss same-path untracked changes. Treating cue count
as consumers or builder duration as full CLI cost would change the measurement
boundary. These alternatives do not satisfy the approved contract.

## Verification Or Enforcement

Canonical M1 T1-T22 specify contract, real Git/SQLite/CLI, failure, concurrency,
privacy, history, calculation, and distribution checks. Their observed results
are recorded in the canonical plan, separately from this accepted decision.
