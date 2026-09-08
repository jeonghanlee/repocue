# RepoCue Evaluation

## Objective

The evaluation system measures whether maintained RepoCue context reduces the
total resources required for an agent to understand a repository's current
state. Core packages remain independent of a specific agent, model, or
tokenizer.

## M2 Conditions

M2 uses five fixed conditions under the same repository state, working
directory, model, reasoning setting, sandbox, benchmark, output schema, and
maximum estimated context tokens:

| Condition | Context |
| --- | --- |
| `direct` | No RepoCue context |
| `placebo` | Branch, HEAD, dirty state, snapshot, and freshness |
| `basic` | Frozen `repocue/1` overview |
| `ranked` | Ranked `repocue/cue-2` repository facts |
| `structural-oracle` | Ranked facts plus external structural candidates |

All assisted conditions receive the same instruction: use supplied context
first and inspect the repository whenever it is incomplete, uncertain, or
insufficient. They are not told to avoid fallback reads.

### Experimental Ranked Selection

The `ranked` and `structural-oracle` conditions share the same deterministic
ranked-content fitter. This policy is experimental and does not change the
frozen `basic` condition.

Ranked content admits at most 12 document paths. Root README, AGENTS,
ARCHITECTURE, and CHANGELOG files rank ahead of nested README files; remaining
ties use path depth and bytewise path order. Directory candidates are ordered
as depth one followed by depth two. Budget fitting removes depth-two
directories first, then low-ranked documents, Makefile targets, recent commit
subjects, and finally depth-one directories before entry points or structural
candidates. The four-repository M2 corpus must retain all depth-one directories
at the 500-token experiment budget.

## Preparation and Isolation

The harness prepares one deterministic full baseline, one no-op refresh, and
all condition contexts before starting an agent. Every report carries the same
maintenance ID and RepoCue snapshot for that prepared state. Baseline and
refresh cost are counted once in reuse analysis.

Each individual scan assumes files and Git state stay unchanged until that
scan returns. The initial Git basis is captured once. Separate checks around
external agent and oracle execution remain required, because those processes
can span repository state transitions between scans.

Live Git facts and structural-oracle output are bracketed by snapshot checks.
The harness stops if the repository no longer matches the maintained snapshot.
The output directory and temporary workspace parent must be outside the
evaluated worktree and Git directory so experiment artifacts cannot change a
later condition.

Each condition starts a fresh external runner process. The harness captures a
state fingerprint before and after the process and rejects a run that modifies
the repository. All five reports are written under a temporary report-set
directory and published by one directory rename after every condition passes.
A failed run exposes no part of the set and can be retried with the same index.

The observed experiment uses three runs for every repository
and condition. A complete comparison group therefore has 15 reports per
repository. Per-repository results are reported before any aggregate.

## Versioned Contracts

- Condition report: `repocue/evaluation-4`
- External runner observation: `repocue/evaluation-runner-4`
- Benchmark: `repository-state-v2`
- Benchmark answer: `repocue/benchmark-answer-2`
- Experimental cue: `repocue/cue-2`
- Qualitative assessment: `repocue/qualitative-assessment-2`

Each published directory is named by repository, model, run index, and
maintenance ID. Files within it are named by repository, condition, model, and
run index. They record the supplied repository path and resolved root, exact
HEAD, state fingerprint, RepoCue snapshot, condition order, cue bytes,
estimated cue tokens, runner metadata, token components, commands, final
response, and limitations.

The runner observation contract requires ordered usage events and one final
JSON object. Runtime validation rejects negative values, token or command
totals that contradict their observations, unknown or inconsistent status
values, basis metadata mismatches, and runner metadata drift across conditions.

New executions require version 4. Calibration import also accepts unchanged
`evaluation-3` reports with their matching `evaluation-runner-3` contract.
Import selects the declared JSON Schema and runtime rules; unknown versions,
mixed report versions within one selected set, mismatched report/runner
versions, and incomplete version-3 token observations are rejected.

Legacy reads preserve source bytes, digests, versions, and declared values.
They cannot determine whether a historical zero originated from an omitted
Codex field. Legacy report readability does not establish applicability:
producer, repository, cue, and evidence compatibility checks still apply.

## Codex Adapter

`repocue-codex-runner` invokes a fresh ephemeral Codex non-interactive run in
JSONL mode with ignored user configuration and rules, read-only sandboxing,
the selected model and reasoning setting, and benchmark output schema.

Adapter version 3 preserves every `turn.completed` event, including a turn
whose usage object is absent, null, or empty. Each token counter is optional:
an omitted or null value means unknown; an explicit zero remains observed.
The output omits unknown counters and requires `not_observed` for each
unknown aggregate. Version-4 readers also accept explicit JSON null.

A counter's aggregate is available only when every completed turn supplies
that counter. Partial values remain in the ordered usage events but never
masquerade as a complete run total. Input and output determine total tokens;
missing cached or reasoning values do not erase a complete input/output total.
Cached and reasoning counts are subsets and are never added to total tokens.
Negative values, invalid event order, cached counts exceeding a known input
count, and integer overflow are errors. A stream with no completed turns is
an error. A completed turn without usage can still retain commands, duration,
the final response, and correctness. Only comparisons and projections
requiring missing measurements become unavailable.

The adapter distinguishes measurement sources:

```text
observed
derived
estimated
not_observed
```

Token events, completed tool events, execution duration, and the final agent
response are observed from the process and event stream. Git/search command
classes and explicit path names are derived from command text.

`named_file_size_proxy_bytes` is the sum of current file sizes for repository
paths explicitly visible in classified command text. The assisted form is
`fallback_named_file_size_proxy_bytes`. These are derived proxies. They do not
measure bytes transferred by Git, directory traversal, kernel cache behavior,
partial reads, repeated reads, or actual filesystem I/O.

## Benchmark and Scoring

Repository-state v2 preserves the deterministic Git questions from v1 and
adds project-defining symbols, signatures, owners, and relevance. The
deterministic scorer compares branch, full HEAD, dirty state, tracked change
paths, and untracked paths with the snapshot basis.

Purpose, architecture, documentation, recent changes, structural completeness,
and uncertainty handling use a separate blinded qualitative assessment. The
assessment records assessor identity, response ID, repository HEAD, state
fingerprint, run index, condition, scores, and evidence. Qualitative values are
not presented as deterministic ground truth.

## Structural Oracle

The M2 oracle uses Bash, Git, grep, awk, and sort. It extracts conservative
Bash functions and source relationships plus Python classes, functions,
methods, signatures, and imports. Production `src/**/*.py` candidates rank
before `tests/**/*.py`.

The oracle has no ctags, Python runtime, Tree-sitter, or parser dependency. It
does not infer call graphs, data flow, or references. Unsupported records are
omitted and reported as limitations.

## Reuse Analysis

Reuse projections cover 1, 2, 3, 5, and 10 independent consumers and keep
these values separate:

- one-time baseline cost;
- incremental refresh cost;
- per-agent cue cost;
- per-agent fallback discovery cost.

M2 experiment reuse totals are `derived`. Estimated cue tokens remain the fast
runtime estimate `ceil(serialized UTF-8 bytes / 4)`. Measured tokenizer-
specific counts are separate fields.

M2 does not claim success before real condition reports and blinded
qualitative assessments exist. Dry runs verify contracts and execution paths;
they are not agent evidence.

## Quantitative Reporting

Routine `repocue report` separates recorded maintenance from calibration-based
agent projections. It runs no agent, model, or tokenizer. Without calibration
it reports maintenance and unavailable agent evidence, not savings. Local M1
adds no real-agent result: its synthetic external-runner fixtures validate
software behavior only.

Direct/basic preparation times the shipped cue CLI through exit and complete
stdout capture, then delivers those exact bytes to the basic runner. Evidence
records executable/revision, arguments, cache/hint state, output digest, times,
and digest-linked Git Trace2 events. Builder time remains diagnostic; setup
and pre/post isolation captures are separate experiment overhead.
Import rejects a cue observation that finishes after the selected assisted
pre-run capture; matching bytes alone do not establish pre-agent delivery.

Import requires complete pairs with identical repository and runner contracts
and verifies content-sensitive evidence against the checkout and maintained
snapshot. One pair is `exploratory`; at least three are `descriptive`, never a
significance claim. Unsupported legacy provenance is rejected; optional missing
cue cost or scoring stays unavailable. Raw answers and narratives stay external.
Estimated source agent costs remain labeled diagnostics. Only observed or
derived agent costs enter paired calibration statistics and reuse arithmetic;
an excluded estimate increases unknown coverage only for its measurement.

The existing scorer checks branch, HEAD, dirty state, tracked changes, and
untracked paths independently of qualitative assessment. Incorrect valid
responses remain in efficiency samples. Optional assessment-2 scores are
digest-joined to responses and retain assessor and known dates. Missing dates
are not inferred from file timestamps; no score assesses a later snapshot.

For pair `i`, `D_i` is direct agent cost, `A_i` assisted agent cost, `Q_i`
comparable full cue CLI cost, and `H` recorded local epoch maintenance:

```text
direct_i(N) = N * D_i
assisted_i(N) = H + N * (A_i + Q_i)
saving_i(N) = direct_i(N) - assisted_i(N)
```

Calculate per pair before medians, ranges, and percentages. Paired
`[100, 200, 300]` and `[90, 1, 299]` have median difference 10, not the
difference of medians, 110. Missing pairs have per-metric coverage. Future
1, 2, 3, 5, and 10-consumer scenarios are `estimated` even at an exact measured
reference. `--consumers N` adds an explicit scenario; cue counts are not users.

Agent tokens already include supplied context, so additional cue tokens are
zero by accounting definition; estimates are not added again. Deterministic
core H has zero LLM tokens. Runtime includes full CLI Q and full recorded epoch
baseline/refresh H once, not their already-included scan portions. For positive
`g_i = D_i - (A_i + Q_i)`, the first strictly cheaper count is
`floor(H / g_i) + 1`. Other gaps have no finite break-even; missing comparable
components suppress only the affected total.

Command classifications are not Git processes. Named-file-size proxies are not
scan bytes or filesystem I/O. No net I/O savings, task-ready latency, CPU,
memory, freshness lag, historical DB size, or routine cue-runtime trends are
invented. Current validation cost is visible separately. See
[CLI usage](CLI.md#quantitative-reports) for import, cutoffs, and external
periodic publication. Fresh real-agent calibration remains separately
authorized; reporting installs no scheduler and launches no model.
