# Task context benchmark reference

## Scope

Version 1 compares the existing overview with original task context. This
reference defines task inputs, scoring, fixture limits, and paid preflight.

**Out of scope:** direct/basic calibration, global skills, automatic model
selection, automatic retries, and continued-conversation inference.

## Original task specifications

The examples use the [tracked declaration](../task-context.example.json).

| Task | Specification | Success evidence |
| --- | --- | --- |
| Scoped explanation | [explanation.json](task-context-v1/explanation.json) | Accurate cited claims and a bound independent assessment |
| Applicable references | [discovery.json](task-context-v1/discovery.json) | Exact selector-derived required IDs and references |
| Required verification | [verification.json](task-context-v1/verification.json) | Frozen command, complete actual output, and matching exit |

These are local preparation examples. They do not authorize paid execution.
Exact paid sources, thresholds, rubrics, and assessors must be accepted before
model outcomes exist.

## Paired inputs and retained outputs

A is the frozen overview; B is `repocue/task-cue-1`. Both receive the same task,
declaration path, targets, checks, model, reasoning, speed, read-only setting,
and answer schema. Both can inspect original sources. Actual cue sizes can
differ; the common estimate budget stays fixed.

Each invocation prepares both actual CLI paths and optionally runs one pair.
Unique `task-pair-*` directories retain `benchmark.json`, answer schema, optional
rates, maintenance/cue costs, `manifest.json`, and attempt directories.

The pair records `output_schema_digest` from the actual `answer.schema.json`
bytes before attempts. Each runner request binds that digest and pair schema
path. The adapter checks the bytes, retains an attempt-local `answer.schema.json`,
and supplies that captured file to the provider's `--output-schema` argument.

Attempts retain `attempt-start.json`, `attempt.json`, cue, prompt, request,
submitted prompt, captured answer schema, invocation, provider JSONL/stderr,
and runner stdout/stderr.
Incomplete journals remain visible. Raw partial usage never becomes a full total.

Summary resolves its input directory to the absolute physical directory.
It validates frozen inputs, digests, captured transport, and the answer schema's
pair/request/capture/invocation identities before recalculating metrics and
outcomes. Missing or changed schema evidence keeps attempts visible and
prevents a complete comparison. Optional assessment bundles have
`repocue/task-evaluation-1` and kind `assessments`. Assessments bind task, policy,
attempt, answer, fingerprint, assessor, rubric, and original references.

## Credit and quality calculation

Rates require ID, nonfuture `as_of`, model, `speed_tier`, unit
`credits_per_million_tokens`, and input/cached-input/output rates.
The formula is `((input-cached)*input_rate + cached*cached_rate + output*output_rate)/1000000`.

Input includes context. Cached input and reasoning output are subsets.
Missing reasoning alone preserves otherwise complete credit inputs.
Missing counters, incomplete turns, incompatible rates, subset violations,
and overflow make the estimate unavailable.

Wrong and incomplete attempts remain visible. Pair differences are overview
minus task; medians and ranges summarize matched differences. Known/unknown
coverage belongs to each metric. Credit estimates are not account billing.
A verdict needs successful required tasks, complete required delivery, and
the frozen policy's exact pair count.

## Fixed paid comparison schedule

| Task | Repeat 1 | Repeat 2 | Repeat 3 |
| --- | --- | --- | --- |
| explanation | A/B pilot | B/A | A/B |
| discovery | B/A | A/B | B/A |
| verification | A/B | B/A | A/B |

The two-execution pilot is the first explanation pair. Continuation requires
explicit approval. Nine pairs contain 18 executions including the pilot.
Order imbalance stays visible: 2:1 per task and 5:4 overall.
Three repeats support descriptive values only.

## Paid experiment preflight conditions

Before G3 authorizes the pilot, freeze:

- Reviewed revision, HEAD/fingerprints, declaration, source/check pins, task
  files, answer schema, and common cue budget.
- Provider executable/version, model, reasoning, speed, sandbox, and observed
  support for required usage/completion/output fields.
- Digest-bound success/rubric, independent assessor, exact pair/evidence
  coverage, and treatment of missing semantic evidence.
- Dated applicable credit rates and external evidence location.
- Authorized execution count, spending stop controls, and pilot continuation.

One pair bounds execution count, not charges. The adapter has no hard billing
ceiling. Unsupported evidence can prevent a complete conclusion.
Local software approval does not resolve G3.

## Local verification boundary

Tests build the shipped CLI and runner, use real Git/SQLite, and substitute
only external Codex. Verification fixtures execute the declared Go test.
Failed arms, malformed output, cancellation, mutations, missing evidence,
wrong commands, partial output, and cheaper wrong answers use the real path.

Synthetic usage, completion flags, rates, and assessment judgments verify
software handling. They establish no actual provider support, charge, agent
compliance, savings, or continued-conversation benefit.
