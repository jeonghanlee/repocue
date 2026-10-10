# Explicit prompt-cache evaluation

## Scope

`evaluate-cache` prepares original task context, sends individually selected
Claude Messages requests, and summarizes retained observations. It is for
discovery and explanation tasks with a tracked task-context declaration.

**Out of scope:** tools, streaming, command execution, automatic retries,
background requests, Codex credits, account billing, and automatic paid runs.
The [task contract](TASK_CONTEXT.md) defines source pins and selection.

## Data flow

Preparation invokes the actual refresh and task-cue CLI against an initialized
cache. It validates original sources and repository basis, then freezes the
benchmark, cue, answer schema, and exact requests in a unique external directory.
Preparation requires no API key and sends no provider request.

Each request contains fixed system instructions and two user text blocks. The
first holds declaration identity, sorted selected items, and original sources
once. Explicit cache steps put `cache_control` with the frozen TTL on this
block. Baseline steps omit it. The second holds task and current Git/freshness/
budget metadata. Source bytes are unchanged. Full wire bytes must fit the
benchmark's estimated context budget; required content is never truncated.

Reusable identity includes the model, API version, TTL, system instructions,
output schema, fixed thinking/effort settings when present, output bound,
and first block. Changing only task/Git metadata
preserves that identity. Changing selected content or settings changes it.
This local identity does not prove provider eligibility or a cache hit.

Run reserves one attempt directory before checking current source bytes and
basis. It reads `ANTHROPIC_API_KEY` only after local preflight, then sends one
non-streaming HTTPS POST. It records request-write evidence, bounded raw
response, nullable HTTP details, wall time, and post-request basis. Failed or
interrupted attempts remain visible and cannot be retried in that directory.

Observation `started_at` and `finished_at` contain the whole attempt, including
Git/source preflight and postflight. Separate nullable `http_started_at` and
`http_finished_at` retain the actual adapter interval; `duration_ms` derives
from those times. A failure before HTTP leaves all three HTTP values null.
Summary validates attempted state, containment, and duration consistency.
For TTL evidence, use the HTTP request start associated with observed cache
write/read usage as the latest observed access, rather than the journal start
or response end. This does not automate waiting or establish provider expiry.

Summary reads frozen files only. It reconstructs requests, validates digests,
and recalculates usage, answer quality, and costs without Git, subprocesses,
network, credentials, or evidence writes. Digests establish local consistency;
they are not provider signatures or proof against coordinated evidence edits.

## Commands

Start from an initialized idle checkout with a tracked declaration. Adapt the
[benchmark example](benchmarks/prompt-cache-v1.json) to its real paths and
selection. The example has no rates and does not establish current pricing.

```bash
benchmark=/absolute/path/to/cache-benchmark.json
evidence=/absolute/path/outside/the/evaluated/checkout
repocue evaluate-cache prepare --benchmark-file "$benchmark" --output-directory "$evidence"
```

Preparation accepts `--repository`, `--cache-dir`, and `--cue-binary`. The
default cue executable is the current binary. Output identifies the unique
prepared directory. An uninitialized cache fails instead of initializing it.

Only an explicit run contacts the provider. Set the API key in the caller's
environment; keep it out of shell history and benchmark files. Run steps in
their frozen order, selecting exactly one per invocation:

```bash
prepared=/absolute/path/to/the/prepared/cache-eval-directory
repocue evaluate-cache run --input-directory "$prepared" --step baseline-a
repocue evaluate-cache run --input-directory "$prepared" --step baseline-b
repocue evaluate-cache run --input-directory "$prepared" --step cache-a
repocue evaluate-cache run --input-directory "$prepared" --step cache-b
repocue evaluate-cache summary --input-directory "$prepared"
```

The default endpoint is `https://api.anthropic.com/v1/messages`.
`--endpoint` permits an explicit HTTPS loopback TLS fixture. Other endpoints,
non-HTTPS URLs, credentials in URLs, queries, fragments, and redirects fail.
No application retry, follow-up request, or automatic expiry wait exists.
Changed-prefix experiments use separately prepared changed source content;
expiry probes require the operator to wait outside RepoCue. Conditions are
intentions and never override observed usage.

## Contracts and limits

| Contract | Version | Purpose |
| --- | --- | --- |
| cache-benchmark | 1 | Frozen selection, schedule, provider settings, limits, and rates |
| cache-request | 1 | Exact wire body and spec/cue/schema/basis/prefix digests |
| cache-observation | 1 | Start/final journal and HTTP/basis/response evidence |
| cache-answer | 1 | Required item/source IDs, cited claims, and uncertainties |
| cache-evaluation | 1 | Preparation report, inventory/summary, or assessment bundle |

All fields are explicit. Unknown fields or versions, duplicate JSON members,
trailing JSON, invalid paths, inconsistent usage, or incompatible rates fail.
Provider responses permit extension fields while duplicate members and unsafe
counters still fail. The [answer example](benchmarks/prompt-cache-answer-v1.json)
is illustrative schema data, not a captured provider answer.

The supported local model profiles are `claude-sonnet-5-5`,
`claude-sonnet-4-6`, `claude-opus-4-6`,
`claude-opus-4-7`, `claude-sonnet-4-5-20250929`,
`claude-opus-4-5-20251101`, and `claude-haiku-4-5-20251001`.
They use API version `2023-06-01` and explicit `5m` or `1h` TTL. This list is
a bounded adapter profile, not a live availability guarantee. Provider-specific
minimum cache sizes and model output limits still apply.

Sonnet 5.5 freezes `thinking.type=between_tools` and
`output_config.effort=high` alongside `output_config.format`. Without tools,
this thinking setting returns text [1]. Neither setting is a public benchmark
control. Older profiles omit both fields, preserving their request bytes and
reusable identities. An unexpected thinking block fails the text-only answer
contract; complete compatible usage and its cost remain visible independently.
Local TLS verification does not establish provider availability or cache reuse.

| Software bound | Allowed range |
| --- | --- |
| Scheduled requests | 1-32; no more than max_requests |
| Maximum output tokens | 1-65536 |
| Timeout milliseconds | 1-300000 |
| Retained response bytes | 64-8388608 |
| Full wire context estimate | 1-2000000; UTF-8 bytes divided by four, rounded up |

These bounds are not spending guarantees. A credential-echoing response body
is withheld. External artifacts contain repository source text and raw answers;
they are distinct from content-free SQLite snapshots.

## Accounting and quality

Ordinary input, cache writes, cache reads, and output are disjoint. The cache
write aggregate validates the lifetime parts but is not added again. Both
lifetime parts must be consistent; absent parts are allocated only when the
single explicit TTL makes the allocation unambiguous. Missing counters are
unknown, including when other categories are observed zero. Optional thinking
usage is a subset of output and is not charged twice.

Rates are optional decimal strings in `usd_per_million_tokens`, with an explicit
record ID, nonfuture date, exact model/API version, ordinary input, 5m write,
1h write, read, and output prices. Supply rates appropriate to the actual
provider pricing conditions. RepoCue applies no universal cache discount and
does not fetch prices. Estimates are not invoices. A response-model mismatch
or incomplete required usage suppresses cost. Known cost of an unsuccessful
task remains in inventory; unknown attempts suppress the complete total.

A pair matches ordered baseline/cache prompts under frozen settings and basis.
Differences include the entire cycle, initial creation, and output. Probes are
separate. A savings verdict additionally needs positive observed cache-read
usage, successful source delivery, correctness, and independently assessed
compliance for every paired answer. Lower cost alone is insufficient.

Each matched pair also separates `initial_request_usd` (the first cache-arm
request, including any observed creation), `subsequent_requests_usd` (all later
cache-arm requests), and `observed_reuse_count` (requests with positive cache
read). These values do not assume the initial request was cold. An unknown
read counter makes the reuse count unavailable; no subsequent requests makes
their total unavailable. Full-cycle cost never omits the first request.

Discovery checks the exact selected required item/source sets. Claims must
cite valid delivered sources. Explanation correctness and instruction compliance
need external independent assessment. `summary --assessment-file` accepts a
cache-evaluation-1 assessment bundle bound to each step's benchmark, request,
raw response, normalized answer, source IDs, basis, assessor, and rubric. An
assessment cannot override failed deterministic checks or rejected bindings.

HTTP wall time is observed separately from preparation process costs. Account
billing, CPU, memory, filesystem I/O, task-ready latency, and time to first token
remain `not_observed`. Local synthetic TLS fixtures verify these software
rules; real cache activity and savings require actual authorized provider runs.

## Codex CLI companion measurement

Use the existing `evaluate-task` overview/task comparison for Codex CLI.
It records total input, cached input, output, optional reasoning output,
dated credit estimates, cue/adapter duration, supported command events,
source delivery, and independently assessed task correctness. See
[Evaluation](EVALUATION.md#independent-task-comparison) and the
[task benchmark reference](benchmarks/task-context-v1.md).
Codex credits have no separate cache-write charge; API-key usage has its own
pricing [2]. Claude USD estimates and Codex credits remain separate populations.

Prepare the tracked explanation benchmark without selecting or running a model:

```bash
task=docs/benchmarks/task-context-v1/explanation.json
evidence=/absolute/path/outside/the/evaluated/checkout
repocue evaluate-task --task-file "$task" --output-directory "$evidence"
```

Use an already initialized maintained cache. Without `--runner`, preparation
freezes actual refresh/overview/task outputs, timings, source pins, policy,
answer schema and repository basis in a unique child. Record the actual
RepoCue executable's path and digest alongside that evidence; preparation
does not automatically retain executable identity.
It does not capture submitted stdin until an explicit runner attempt.

RepoCue supplies CLI stdin, model/reasoning/speed arguments and the final-answer
schema. The CLI controls its hidden context and provider calls. The existing
task-first prompt includes the complete cue; this path adds no explicit cache
boundary, TTL control, full provider-wire capture or output-token cap. Two CLI
executions do not bound HTTP request count or charges. Cached-input counters
do not identify which original source was reused. A smaller cue can reduce
both input and cached tokens; compare complete paired credits and quality
instead of maximizing cache hit ratio.

Paid execution requires separately frozen model/settings, CLI version,
applicable dated credit rates, independent assessment policy and spending
conditions. Start with one authorized explanation A/B pair, then stop for
the continuation decision. The full schedule remains nine pairs and eighteen
executions including the pilot. No-model preparation establishes input
readiness only; it establishes no provider behavior, correctness or savings.

## References

[1] Anthropic. "Claude Sonnet 5.5 migration guide." Claude Platform Docs. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/models/sonnet-5-5/migration-guide

[2] OpenAI. "Pricing." ChatGPT Learn. Accessed: Oct. 9, 2026. [Online]. Available: https://learn.chatgpt.com/docs/pricing
