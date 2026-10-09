# ADR: Original task context and retained evidence

## Scope

This decision defines task context delivery and local evaluation independently
of repository-state calibration and paid-model authorization.

Date: 2026-10-08
Status: accepted
Source Decisions: D25-D30 in docs/milestone-10a4f02.md

## Context

Overviews identify state but do not deliver applicable instruction or procedure
text. Small output cannot establish task readiness, correctness, or credit
savings. Command classification cannot prove source provision or check success.

## Decision

Use independent declaration, cue, benchmark, answer, runner, and result
versions. Keep legacy versions, SQLite schema, and content-free snapshots.
Implement deterministic selection in Go: null dimensions are global; lists
use OR within dimensions and AND between them.

Pin tracked regular declaration and source files to full-file snapshot digests.
Support whole files or inclusive one-based ranges. Preserve original UTF-8,
line endings, and final newline. Reject linked components and escaping paths.
Stream hashing while bounding retained text. Keep the quiet-repository assumption.

Assemble required primary/check sources and provenance before whole optional
units. Mandatory overflow fails; optional omissions remain explicit.
Cues declare coverage without enforcing instruction authority or permissions.

Separate prompt/tool provision, task correctness, verification execution, and
independent semantic assessment. Capture actual stdin and supported completion,
exit, and output evidence. Unknown fields prove nothing. Explanation assessment
binds the exact answer, source basis, and policy.

Prepare without an agent by default. Explicit execution performs one ordered
overview/task pair. Retain external attempts before launch and raw evidence on
failure, malformed output, cancellation, and basis errors. Summaries launch no
process and retain wrong answers.

Use complete compatible counters and dated credit/speed rates. Validate subset
bounds, absence, and overflow. Calculate matched differences before descriptive
aggregates. Suppress verdicts without pre-execution policy, successful tasks,
complete required delivery, and required pair coverage.

## Consequences

Authors maintain applicability and renew changed pins. Original context can
cost more than summaries; budget failures expose missing mandatory material.
External evidence requires owner retention. Local fixtures prove software
behavior, not provider support, model compliance, account billing, or savings.
Paid runs require separate settings, policy, rates, spending, and continuation
authorization.

## Alternatives considered

Whole-file-only delivery increases input. Contained symlinks need separate
link/target identities. Summaries introduce model cost and uncertain fidelity.
Shared legacy calibration mixes incompatible task outcomes. Success-only
publication hides failures and favorable sample selection.
These alternatives do not meet the accepted slice.

## Verification or enforcement

Canonical M4 / T1-T8 cover contracts, real source integrity, budgeting, shipped
CLI/runner paths, retained failures, regressions, distribution, and independent
acceptance. Observed results belong in the canonical milestone.
