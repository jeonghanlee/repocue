# ADR: Explicit provider prompt caching and retained measurement

## Scope

This decision defines optional Claude Messages requests over original task
context, independently of legacy calibration and task-runner contracts.

Date: 2026-10-09
Status: accepted
Source Decisions: D26-D30 in docs/milestone-10a4f02.md

## Context

Repository context can be reused across tasks, but a small cue alone does not
establish a provider cache hit, lower cost, correct answers, or compliance.
Provider usage categories and pricing differ from Codex credit accounting.

## Decision

Keep the core local and deterministic. Add independent versioned benchmark,
request, observation, answer, and evaluation contracts without changing SQLite
or legacy schemas. Preparation runs actual refresh and task-cue commands and
freezes source identities, provider settings, limits, rates, and request bytes.

Place original selected UTF-8 sources once in a stable user content block.
Keep task, Git basis, freshness, budget, omissions, and warnings after its
explicit cache boundary. Repository text does not acquire system authority.
Include all preceding settings and output schema in reusable identity.

Freeze model-specific thinking and effort along with model, API version, TTL,
system instructions, output schema and output bound. Sonnet 5.5 uses
`thinking.type=between_tools` and `output_config.effort=high` for the existing
text-only, tool-free contract. Preserve `output_config.format`. Other supported
profiles omit the new fields and retain their exact wire bytes and reusable
identities. Unknown profiles fail before HTTP. Unexpected thinking blocks fail
answer validation without discarding complete compatible usage or its cost.

Use standard Go HTTP for one explicit bounded non-streaming Messages request.
An exclusive attempt directory and start journal precede execution. Validate
source bytes and repository basis before send and basis after send. Retain
failed, incomplete, and interrupted attempts. Do not retry or follow redirects.
Read the API credential only during explicit run and exclude it from evidence.

Normalize ordinary input, cache creation by lifetime, cache read, and output
as disjoint nullable counters. Creation aggregate is not added to its parts.
Estimate USD with exact decimal arithmetic and dated compatible supplied rates.
Unknown usage suppresses dependent totals. Failed answer cost stays visible.

Reconstruct frozen requests and recompute raw observations in a read-only
summary without subprocesses or network. Compare full matched cycles including
initial writes and outputs. Savings verdicts require observed cache reuse,
complete compatible costs, successful delivery, correctness, and bound
independent compliance assessments. Discovery validates exact source/item sets;
explanation also requires semantic assessment.

## Consequences

Explicit run sends selected original repository text to the chosen provider.
Source authors maintain pins. Evidence lives outside the evaluated checkout
and needs owner retention. Local TLS fixtures establish software behavior,
not provider cache support, actual hits, account billing, or real savings.
Paid experiments require separately selected settings and spending authority.
The default build remains CGo-free and adds no mandatory runtime dependency.

## Alternatives considered

Automatic cache calls obscure spending and failure counts. Shared legacy
schemas conflate different token categories and quality criteria. Summaries
lose original-source fidelity. Success-only evidence hides unsuccessful cost.
These alternatives do not satisfy the accepted measurement boundary.

## Verification or enforcement

Real CLI integration exercises Git, SQLite, extraction, assembly, HTTP,
retention, and summary with only the outer TLS server substituted. Contract,
accounting, race, distribution, and documentation checks enforce local behavior.
Real-provider observations and milestone evidence are recorded separately.
