# Task context contracts

## Scope

A task cue delivers author-declared original repository context for an explicit
task and target set. This reference defines applicability, source identity,
budgeting, and evidence boundaries for agents and declaration authors.

**Out of scope:** runtime instruction priority, permissions, global skill
loading, semantic summaries, source-command execution, and paid experiments.
Delivered instructions remain subject to the agent runtime's own rules.

## Versioned public values

| Value | Version | Schema |
| --- | --- | --- |
| Declaration | repocue/task-context-1 | task-context-v1.schema.json |
| Task cue | repocue/task-cue-1 | task-cue-v1.schema.json |
| Benchmark | repocue/task-benchmark-1 | task-benchmark-v1.schema.json |
| Answer | repocue/task-answer-1 | task-answer-v1.schema.json |
| Runner request and observation | repocue/task-runner-1 | task-runner-v1.schema.json |
| Preparation, pair, attempt, summary, and assessments | repocue/task-evaluation-1 | task-evaluation-v1.schema.json |

All schemas are embedded locally under `docs/schema/`. Unknown fields,
duplicate JSON members, trailing values, and invalid UTF-8 fail strict parsing.
Runtime validation also checks IDs, paths, ranges, tracked membership, and
digest relationships that JSON Schema cannot establish. Existing overview and
calibration contracts retain their versions. Task results cannot enter the
direct/basic calibration importer.

## Declaration and applicability rules

The [example declaration](task-context.example.json) pins the repository
README. Its digest must be renewed explicitly after the README changes.
The declaration itself must be tracked and match its snapshot digest.

| Field | Meaning |
| --- | --- |
| task_kinds | Distinct, nonempty registry of recognized task names |
| items[].id | Unique, stable item identity |
| kind | instruction, design, or verification |
| required | Applicable item and all its sources must fit |
| task_kinds | Null means global; a nonempty list matches any listed task |
| path_prefixes | Null means global; a nonempty list matches any listed component prefix |
| source | Original primary source reference |
| checks | Original supporting procedure references; no execution implied |

The dimensions combine with AND; each list combines with OR. Multiple targets
select the union. `cmd` matches `cmd/main.go`, but not `cmdline/main.go`.
An existing target directory also covers descendant item scopes. A lexical
target for a future file is valid. Explicit `.` covers the repository.

Paths are canonical relative slash paths with exact case. Empty, absolute,
drive-qualified, backslash, control-character, traversal, and noncanonical
paths fail. Missing task/target and unknown task names fail. Targets are
sorted and deduplicated; items are ordered by ID.

RepoCue preserves conflicting instructions as distinct items. It does not
resolve authority or infer applicability from prose. Coverage is the declared
set, not every instruction a runtime can load.

## Original source identity

Each reference contains `path`, full-file SHA-256 `content_digest`, and
`lines`. Null selects the whole file. Otherwise `start` and `end` are
inclusive, positive, one-based line numbers with `end >= start`.

A selected line includes its original LF delimiter. CRLF bytes remain CRLF.
An unterminated final line stays unterminated. Empty whole files are valid;
ranges beyond actual lines fail. Invalid UTF-8 anywhere in the full file fails,
including bytes outside a selected range.

Expected, snapshot, and actual full-file digests must agree. Extracted text
also has `text_digest`. Source IDs hash path, full-file digest, and range.
Identical references share one source record; every item retains its primary
and check-source IDs.

Declaration, source, and intermediate-component symlinks fail. Gitlinks,
untracked files, deleted entries, and nonregular files fail. Reads use a
contained root, metadata checks, and streamed full-file hashing. Refresh never
silently renews pins or reinterprets ranges.

The repository must remain idle during scans. Pre/post extraction checks do
not provide a race-free repository-wide snapshot. See the
[scan assumption](ARCHITECTURE.md#transaction-boundary).

## Whole-unit budget and failure rules

The CLI requires `--view task`, `--context-file`, `--task-kind`, and repeatable
`--target`. See the [CLI reference](CLI.md#task-context-and-evaluation).

`refresh-needed` prevents delivery; matching `dirty-but-indexed` is valid.
The complete envelope includes provenance, required IDs, original primary/check
sources, and optional omission accounting.

Required units precede optional units. Optional units are admitted whole in
item-ID order. Omitted items retain ID, reason `budget`, and count. All selected
source/check references are validated even when optional text cannot fit.
No text or provenance is truncated.

The estimate is `ceil(serialized UTF-8 bytes / 4)`, not a model tokenizer.
Mandatory overflow, unavailable sources, stale pins, invalid references, and
declaration errors return nonzero with no successful task cue stdout.
Retained text is bounded by the output budget; full-file hashing streams.

## Delivery and task outcome evidence

| Evidence | Available condition |
| --- | --- |
| Prompt provision | Captured stdin includes exact frozen cue with validated decoded original text |
| Tool provision | Identified completed tool, exit zero, explicit complete full-file output matching the pin |
| Discovery correctness | Exact required item and primary/check source reference sets |
| Verification correctness | Every unique frozen command has completion, matching exit, and complete expected output |
| Explanation correctness | Original-source citations and a bound independent assessment |
| Semantic compliance | Explicit source-backed assessment bound to answer and policy |

JSON escaping does not change decoded text. Command names, invocation, and final
assertions cannot prove reads/checks. Missing completion, output, or provider
support stays unavailable. Provision does not establish understanding.

Assessments bind task/policy identity, attempt, answer digest, repository
fingerprint, assessor, rubric, and original references. Missing policy or
semantic evidence suppresses the corresponding outcome. Fixture assessments
verify software handling only.

## Retained local measurement boundary

Preparation refreshes an existing cache and measures actual overview/task CLI
stdout and full process duration. Initial baseline cost is unavailable here.
Default preparation invokes no agent.

Explicit runner mode performs one ordered pair outside the checkout.
An immutable attempt-start journal precedes launch. Raw prompt, cue, settings,
provider/runner output/error, and pre/post repository evidence survive failures
and cancellation. Failed second arms retain completed first arms. No automatic
retry or continuation occurs.
The pair freezes the answer-schema digest before launch. Each adapter verifies
and captures those bytes, then supplies the attempt-local schema to the provider.

Summary launches no process. It validates retained input, digests, transport,
and repository compatibility, including frozen and delivered answer-schema
identity, then recomputes outcomes. Wrong and incomplete
answers remain in inventory. Local digests establish consistency, not provider
signatures.

Complete compatible usage and dated credit rates permit an estimate. Cached
input and reasoning output are subsets. Cue content is already input.
Missing full-run data never becomes a partial subtotal or zero.

Matched overview-minus-task differences precede medians and ranges. A verdict
requires frozen policy, successful required tasks, complete required delivery,
compatible complete pairs, and the exact required pair count.
Three repeats provide descriptive results, not significance. CPU, memory,
filesystem I/O, task-ready latency, and billing have no measurement path here.
See the [task benchmark](benchmarks/task-context-v1.md) for paid preflight.
