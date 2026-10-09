# RepoCue Work Register

## Scope

This document is the canonical work register for quantitative reporting,
repository calibration, task-specific context delivery and measurement,
and provider prompt-caching integration.

**Out of scope:** Implementation or external agent execution before their
separate authorization, and every feature listed in the M1 and M2 exclusions.

Release line: master
Milestone index: 10a4f02
Canonical path: `docs/milestone-10a4f02.md`
Canonical branch or ref: master
Git upstream: origin/master
Remote tracker: none

Next session entry point: accept the independently reviewed Sonnet 5.5 support P008-P009 (plan20261009_092451) and the Codex CLI companion P010-P011 (plan20261009_160442), then record their local implementation authority. The User selected Codex CLI alongside Sonnet on 2026-10-09. Codex uses existing M5 overview/task measurement; Claude API uses M6. The two-request G6 pilot and USD 0.10 allowance remain proposed, not authorized, and do not apply to Codex. Completed local M6 P001-P006 authority does not cover these new proposals. No real-provider request or savings observation exists. M5/G3 and M6/G6 remain separately Blocked/Open; publication needs separate authority. M2/G1 is unchanged.

## Milestone

### Work

| Group | ID | Work unit | Type | Status | Ready | Deps | Done when / Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| M3 | M1 | Quantitative savings report contracts and implementation | Milestone | Complete | No | D1-D23 | Landed in a8f650c; the CLI imports a validated calibration, separates observed maintenance from projected savings, and passes M1 / T1-T22; [detail](#m1---quantitative-savings-report-contracts-and-implementation) |
| M3 | M2 | Initial repository calibration and report | Milestone | Blocked | No | M1, M3, G1 | At least three controlled direct/basic pairs produce a reproducible calibration and first report without modifying the evaluated repository; [detail](#m2---initial-repository-calibration-and-report) |
| M3 | M3 | Token usage absence and report compatibility | Milestone | Complete | No | M1, D2, D14, D24 | Landed in 8d61f6e; missing counters remain unavailable; v4 execution and unchanged v3 reads pass M3 / T1-T4; [detail](#m3---token-usage-absence-and-report-compatibility) |
| M3 | G1 | Calibration evidence authorization | External gate | Open | No | | The User authorizes either a verified retained report set or the cost and conditions of new Codex runs; [detail](#g1---calibration-evidence-authorization) |
| Task context | M4 | Task-specific context delivery and local measurement readiness | Milestone | Complete | No | G2, G4, D25-D30 | Landed in edd3dc2; versioned contracts, shipped task cue, source delivery evidence, T1-T8 and post-EOF fixture verification; [detail](#m4---task-specific-context-delivery-and-local-measurement-readiness) |
| Task context | M5 | Task-context agent comparison | Milestone | Blocked | No | M4, G3, D25-D30 | Three controlled task comparisons retain cost, delivery, execution, and correctness evidence without unsupported savings claims; [detail](#m5---task-context-agent-comparison) |
| Task context | G2 | Independent review session configuration | External gate | Complete | No | | User confirmed L size, Mode B, two independent Reviewers and root coordination/implementation; publication remains separately authorized; [detail](#g2---independent-review-session-configuration) |
| Task context | G3 | Task-context experiment conditions and cost authorization | External gate | Open | No | | Authorize exact model, basis, run count, rate record, evidence location, and spending stop conditions; [detail](#g3---task-context-experiment-conditions-and-cost-authorization) |
| Task context | G4 | Named task-context plan acceptance and implementation authorization | External gate | Complete | No | | User accepted plan20261008_052832 and authorized P001-P007 on 2026-10-08; [detail](#g4---named-task-context-plan-acceptance-and-implementation-authorization) |
| Prompt caching | M6 | Claude API prompt-caching integration and measurement | Milestone | Blocked | No | M4, G5, G6, D26-D30 | An opt-in adapter submits reusable context with cache controls and retains verified usage, request duration, cost estimates and task quality; [detail](#m6---claude-api-prompt-caching-integration-and-measurement) |
| Prompt caching | G5 | Prompt-caching plan acceptance and implementation authorization | External gate | Complete | No | | User accepted and authorized local plan20261009_075715; two detailed-contract cross-checks found no blocker; [detail](#g5---prompt-caching-plan-acceptance-and-implementation-authorization) |
| Prompt caching | G6 | Claude API experiment conditions and cost authorization | External gate | Open | No | | Authorize exact provider/model, cache conditions, dated rates, evidence location, run limits and spending controls; [detail](#g6---claude-api-experiment-conditions-and-cost-authorization) |

### Decisions

| ID | Decision | Decision Date |
| --- | --- | --- |
| D1 | Routine reporting invokes no agent, model, tokenizer, or LLM. External runners are used only to produce or renew calibration evidence. | 2026-09-05 |
| D2 | Reports preserve `observed`, `derived`, `estimated`, and `not_observed`; missing values are never converted to zero. | 2026-09-05 |
| D3 | Versioned calibration JSON stores the imported aggregate outside SQLite. SQLite stores only its identifier, path, digest, repository identity, and applicability metadata. | 2026-09-05 |
| D4 | Calibration applicability is `measured_reference` only for its exact basis, `estimated` for a different snapshot under the same compatible contract and epoch with repository-profile drift reported, and `not_applicable` after an epoch or contract change. | 2026-09-05 |
| D5 | A report window is segmented by epoch. Projected totals are not calculated across mixed epochs. | 2026-09-05 |
| D6 | `named_file_size_proxy_bytes` is not combined arithmetically with observed scan bytes and is not reported as filesystem I/O savings. I/O savings remain `not_observed` until comparable observations exist. | 2026-09-05 |
| D7 | Agent execution duration is not named task-ready latency. Task-ready latency remains `not_observed` until a separate operational measurement exists. | 2026-09-05 |
| D8 | A routine report presents fixed projections for 1, 2, 3, 5, and 10 consumers. An explicit consumer count is a scenario, not observed use; cue-run count is never substituted for consumer count. | 2026-09-05 |
| D9 | Generated reports go to stdout and are retained only by an external caller. RepoCue adds no scheduler or report-retention policy in M3. | 2026-09-05 |
| D10 | The first production calibration compares direct discovery with the frozen basic overview cue only. Ranked and structural experiment conditions are not mixed into that profile. | 2026-09-05 |
| D11 | Calibration import excludes agent final responses, prompts, and absolute evaluated-repository paths. Raw evaluation reports remain external evidence. | 2026-09-05 |
| D12 | Break-even is calculated separately for each comparable measurement dimension. It is unavailable when the per-consumer difference is zero or unfavorable and is never reduced to a cross-unit score. | 2026-09-05 |
| D13 | Evaluation snapshot IDs are provenance within their source DB. Import binds verified repository evidence to the active local epoch and snapshot; IDs from different databases are never compared as identity. | 2026-09-05 |
| D14 | Existing evaluation-3 reports remain unchanged. A versioned calibration-evidence JSON file supplies missing provenance and source-file digests; unverifiable legacy evidence cannot produce an applicable profile. | 2026-09-05 |
| D15 | Historical database-size and runtime cue-duration trends remain `not_observed`. M3 adds no measurement-history tables beyond calibration references. | 2026-09-05 |
| D16 | Pairwise differences are calculated before medians. Reuse projections include the local epoch's baseline and all completed refresh costs through the report cutoff, regardless of the activity window. | 2026-09-05 |
| D17 | Reports default to a 30-day UTC window, JSON, and the current repository. They do not refresh the cache or select among multiple profiles automatically. | 2026-09-05 |
| D18 | Optional qualitative-assessment-2 files are joined through the evidence file by response ID, report digest, and repository/run basis. Missing assessments remain `not_observed`; narrative evidence stays external. | 2026-09-05 |
| D19 | Per-consumer cue cost uses the real shipped CLI, including startup, repository/cache access, freshness inspection, serialization, and metric recording. Builder-only timings do not establish CLI duration or Git cost; missing full-path measurements suppress the affected combined projection. | 2026-09-07 |
| D20 | Calibration and reports preserve deterministic Git-fact correctness separately from optional qualitative scores, including incorrect answers, per-response results, paired coverage, and scoring provenance. Missing evidence is `not_observed`; raw answers and expected/actual fact values remain external. | 2026-09-07 |
| D21 | Evaluation, import, and reporting use versioned content-sensitive evidence for non-ignored untracked files. Names-only fingerprints cannot establish exact calibration applicability. The core snapshot and refresh scan scope remain unchanged. | 2026-09-07 |
| D22 | Legacy zero values for scan duration and Git counts without evidence of measurement are `not_observed`, not observed zero. Preserve stored rows and suppress affected totals when coverage is incomplete. | 2026-09-07 |
| D23 | Historical state selection uses recorded observation time, not SQLite publication time. Activity uses each metric row's recording time. Reports expose both boundaries and never claim to reconstruct when a snapshot became available. | 2026-09-07 |
| D24 | New evaluation/runner version 4 distinguishes missing counters from zero and requires complete-turn coverage per aggregate. Legacy version-3 schemas, source bytes, and declared values remain unchanged; imports dispatch by matching declared versions. | 2026-09-08 |
| D25 | Task context selects author-declared repository instructions, design references, and verification procedures by explicit task kind and target paths; it does not infer applicability or summarize meaning with an LLM. | 2026-10-07 |
| D26 | New task-context and measurement contracts are versioned separately; frozen overview, M2 conditions, and direct/basic calibration compatibility remain unchanged. | 2026-10-07 |
| D27 | Source delivery, command completion, result correctness, and semantic compliance are separate evidence classes. A declaration or command string alone proves none of the latter three. | 2026-10-07 |
| D28 | Applicable required context cannot be silently dropped for budget fitting. Missing sources, stale digests, ambiguous source selection, or unresolved applicability prevent a complete task cue. RepoCue does not override execution-tool instruction priority or execute source-declared commands. | 2026-10-07 |
| D29 | Agent comparisons retain all attempts, paired differences, cache observations, and missing measurements. Credit estimates use the applicable dated rate record and do not double-count cached input, reasoning output, or supplied cue tokens. | 2026-10-07 |
| D30 | The local first slice retains Go, SQLite, deterministic extraction, CGo-free Linux and Windows builds, and real Git/SQLite/CLI verification. Real model execution and long-conversation claims need their own observed evidence and authorization. | 2026-10-07 |

### Milestone Details

#### M1 - Quantitative Savings Report Contracts And Implementation

Origin: 10a4f02 / M1
Identity History: none
GitHub Issue: none
Status: Complete

##### Summary

Add a model-neutral reporting path that combines observed RepoCue maintenance
measurements with a separately produced A/B calibration. The report answers
what changed in RepoCue's own cost and what savings are projected for repeated
consumers without presenting projections as observed use.

The canonical machine-readable output is versioned JSON. Markdown is generated
from the same calculation result. Every metric carries its measurement status,
basis, unit, sample count where applicable, and limitations.

##### Scope

- Add a `repocue/calibration-1` JSON contract for compact aggregates imported
  from complete, validated evaluation report sets.
- Add a `repocue/savings-report-1` JSON contract for observed local metrics,
  calibration provenance, applicability, like-for-like trends, and reuse
  projections.
- Add a `repocue/calibration-evidence-1` JSON contract for report digests,
  pairing, producer provenance, repository profiles, full-path cue costs,
  deterministic scoring evidence, and optional assessment references. Preserve
  existing evaluation-3 and qualitative-assessment-2 files.
- Add `repocue calibration import --reports <directory>`.
- Require `--evidence <file>` for import and accept optional
  `--assessments <directory>`.
- Add `repocue report`, `--window <Nd>`, `--format json|markdown`,
  `--profile <calibration-id>`, and `--consumers <positive-integer>` options.
  JSON is the default format.
- Store calibration JSON under the RepoCue cache directory and store only its
  reference and SHA-256 digest in SQLite.
- Compare the selected time window with the immediately preceding equal window
  for like-for-like observed maintenance and cue metrics.
- Prepare direct/basic evaluation pairs through an explicit
  `evaluate-m2 --conditions direct,basic` option. Preserve the existing default
  five-condition behavior and its atomic publication and isolation checks.
- Measure the shipped basic cue CLI before each assisted run and supply its
  output to that agent. Retain the complete command duration and observed Git
  invocation count in external calibration evidence, not new history tables.
- Import and report branch, HEAD, dirty, tracked-change, and untracked-path
  correctness independently of optional semantic assessments.
- Add content-sensitive evidence capture for evaluation, calibration import,
  and reporting without indexing untracked content in the core snapshot.
- Interpret legacy measurement coverage explicitly and define historical
  state selection by observation time using existing stored timestamps.
- Include external `cron` and `systemd` examples that write report stdout to an
  operator-selected destination.

Out of scope: MCP, Claude adapter, production source parsing, Tree-sitter,
embeddings, RepoCue-internal LLM use, checkpoint management, pruning, retention
enforcement, filesystem watching, an internal scheduler, automatic report
retention, and a single cross-unit total-cost score.

##### Completion Criteria

- A complete compatible direct/basic report set imports deterministically into
  a schema-valid calibration artifact with an independently verifiable digest.
- Cross-repository, incomplete, incorrectly paired conditions, mixed-model,
  mixed-benchmark, mixed-budget, or otherwise incompatible selected input is
  rejected without publishing a calibration reference.
- The default report shows fixed 1, 2, 3, 5, and 10 consumer projections and no
  headline claiming observed savings.
- `--consumers N` is labeled as an explicit scenario and never inferred from
  cue runs.
- Token projections use agent-observed direct and assisted totals without
  adding cue estimated tokens a second time. Token components remain separate.
- Wall-duration projections are labeled as execution-duration projections.
  Task-ready latency remains `not_observed`.
- Per-consumer cue duration and Git cost cover the real CLI, including its
  freshness inspection. Builder-only or missing measurements never stand in
  for this cost; affected combined projections remain unavailable.
- Observed scan bytes and agent named-file-size proxies remain separate. No I/O
  savings value is emitted from unlike measurements.
- CPU, memory, filesystem bytes read and written, freshness lag, and any other
  unavailable required metric are present as `not_observed`, not omitted or
  reported as zero.
- Report windows are segmented by epoch. No projection crosses an epoch
  boundary or uses a `not_applicable` calibration.
- Exact-basis, compatible-snapshot, and incompatible calibration states are
  distinguishable from the report alone. Compatible estimates show tracked
  file, byte, and language-profile changes from the measured reference.
- Qualitative results are carried only as dated calibration evidence and are
  never described as an assessment of the current snapshot.
- Deterministic Git-fact results retain each response's pass count, fact count,
  ratio, and per-fact pass/fail values with provenance and paired coverage.
  Incorrect answers remain in both quality and efficiency results; missing
  scoring evidence is explicit and never means a passing score.
- JSON and Markdown contain the same values, measurement classifications,
  limitations, and provenance.
- A break-even consumer count is reported only for a comparable dimension with
  a positive per-consumer difference. Zero, unfavorable, or unavailable cases
  explain why no break-even value exists.
- Reports contain no raw agent response, prompt, or absolute evaluated-
  repository path.
- The default CGo-free Linux static build and Windows amd64 cross-build remain
  available.
- Import matches the complete live repository fingerprint to source evidence
  and the maintained snapshot before atomically binding the profile to the
  local epoch. An identically numbered snapshot in another DB is insufficient.
- Legacy reports without verifiable producer provenance are rejected with the
  missing fields identified; current executable metadata never fills them in.
- Activity-window boundaries, profile selection, pairwise aggregation, and
  maintenance-cost boundaries follow the contracts below.
- Untracked content changes at an unchanged path invalidate exact evidence
  matching even when the core snapshot ID and Git status remain unchanged.
- Legacy placeholder zeros never become observed savings. A historical report
  separates state observation time from metric recording time and makes no
  claim about SQLite publication time.
- Optional assessments have an unambiguous validated join, explicit coverage,
  and recorded assessment time when available. Absent scores or dates are not
  inferred.

##### Repository Binding And Evidence Contract

Import requires an initialized repository and a maintained tracked snapshot
that matches the live repository. Evaluation, import, and report share a
versioned evidence capture covering branch, HEAD, index/status, tracked working
tree, and non-ignored untracked paths and content. The source repository ID and
this content-sensitive fingerprint must match the target checkout; neither
HEAD nor the existing names-only untracked fingerprint is sufficient. M3 does
not automatically transfer calibration between different checkout roots.

For non-ignored untracked regular files, hash content as a stream. Include the
repository-relative path, kind, mode, and content digest in a deterministically
ordered, unambiguously encoded inventory. Hash symlink target text without
following the target. An unreadable or unsupported entry prevents a complete
evidence result; never silently omit it or follow a link outside the checkout.
The evidence scope is Git state plus tracked and non-ignored untracked entries,
not every file on disk. Preserve the existing submodule limitations explicitly.
Apply the idle-repository scan assumption and retain pre/post agent checks.

Store only evidence digests, algorithm/version, coverage, and observation
metadata in calibration/binding data; file contents and detailed inventories
remain external. Keep evaluation-3's original fingerprint as provenance and
carry the stronger fingerprint separately in calibration-evidence-1. Compare
only fingerprints produced under the same algorithm. Do not reinterpret or
overwrite old report or assessment fingerprints. Legacy evidence without
contemporaneous proof of the stronger coverage cannot establish a binding.

This capture belongs to evaluation, import, and report, not ordinary core
refresh or cue generation. It adds no untracked file-state rows or parser.
Record its duration and bytes read in external evaluation evidence separately
from cue CLI cost and agent cost; routine report output identifies its own
current validation work without claiming it is free or adding history tables.

The importer records the source snapshot IDs only as provenance. After evidence
validation, it binds the profile to the target repository ID, active epoch,
local snapshot, and verified fingerprint. The SQLite transaction compares the
expected current epoch and snapshot again before publishing the reference; a
concurrent refresh or rebaseline aborts the import. This prevents a temporary
evaluation DB's `snapshot-000001` from being mistaken for the local DB's state.

For a current report, compare the live tracked basis with the maintained
snapshot and the additional content-sensitive evidence with the calibration
binding. A tracked mismatch reports `refresh-needed` and suppresses projections;
the command does not refresh automatically. Changed untracked content under
the same core snapshot makes the calibration `not_applicable` and suppresses
projections without falsely changing core freshness. Repeating ordinary
refresh alone cannot repair that evidence mismatch; renewed authorized
calibration evidence is required. For an unchanged binding and matching
content-sensitive fingerprint, applicability is `measured_reference`. A new
snapshot in the bound epoch with the same producer/cue contract is `estimated`
and displays the repository-profile changes. A new epoch or incompatible
contract is `not_applicable`; source measurements remain visible as historical
reference values. Model and reasoning settings belong to the selected profile,
not to an inferred active agent session.

`repocue/calibration-evidence-1` names report files relative to `--reports` and
records each SHA-256 digest, pair ID, condition, run index, and source snapshot.
Its file references reject absolute paths, parent traversal, and symlinks that
resolve outside the declared input directories.
Each pair has exactly one direct and one basic report with matching repository
basis, maintenance ID, runner settings, benchmark, and answer schema. Across
pairs, maintenance IDs may differ, but repository basis and execution contracts
must agree. Reject duplicate pairs and duplicate source-report digests.

The evidence file also records the RepoCue source revision and executable
digest, fingerprint algorithm version, cue view/schema/budget and selection
policy version, repository language profile, and the observation times. The
extended evaluation path emits these values when it prepares the reports;
import never derives historical values from the currently installed executable
or a later checkout. Add schema/runtime validation and file-digest checks for
this evidence contract in `internal/evaluation` and the calibration importer.

The direct/basic preparation path records full CLI cue-cost observations and
the contemporaneous Git basis used for deterministic scoring in the external
evidence file, joined to each selected report by digest, pair ID, condition,
run index, and state fingerprint. Nested external evidence references follow
the same relative-path and digest validation rules as report references.

Existing evaluation-3 reports can be used only when retained contemporaneous
records substantiate the missing fields. The evidence file names those records
and their digests. Reject unsupported legacy input without modifying its files
or fabricating a version, language profile, or fingerprint. Old five-condition
sets may supply explicitly selected direct/basic reports through the evidence
file; other conditions are excluded, not aggregated. Inconsistent selected
conditions remain an error. Runnerless dry-run reports cannot calibrate agent
savings.

A single complete pair may be imported with `sample_grade: exploratory`;
projections display that grade. Initial M2 completion requires at least three
complete pairs, six agent runs. Three runs provide descriptive ranges, not a
statistical significance claim.

##### Calculation And Measurement Contract

All arithmetic operates on complete matched pairs for each measurement. If one
side is unavailable, exclude that pair for that measurement, record the reason
and coverage count, and use `not_observed` when no complete pairs remain.
Sample ranges are minimum and maximum, not confidence intervals. Median uses
the arithmetic mean of the two middle values for an even sample count.

For a measurement dimension and pair i, define:

```text
D_i = observed or derived direct-agent cost
A_i = observed or derived basic-assisted agent cost
Q_i = separately measured full basic cue CLI cost, when comparable
H   = local epoch maintenance cost through the report cutoff
g_i = D_i - (A_i + Q_i)

direct_i(N)   = N * D_i
assisted_i(N) = H + N * (A_i + Q_i)
saving_i(N)   = direct_i(N) - assisted_i(N)
```

Calculate these values per pair before reporting medians and ranges. The
headline saving is `median(saving_i(N))`, not a subtraction of independently
aggregated arm medians. Display pair values and sample counts so the result
can be reproduced; independently reported medians need not add up. Percentage
savings are calculated per pair relative to its positive direct total before
aggregation. A zero direct total has an unavailable percentage.

For example, direct costs `[100, 200, 300]` and paired assisted costs
`[90, 1, 299]`, with Q and H equal to zero, have differences `[10, 199, 1]`.
The median difference is 10; subtracting arm medians would incorrectly give
110 for this specified statistic. Retained pairs preserve the association
needed to reproduce the result. Calibration statistics are `derived`; future
consumer scenarios are `estimated`, including at an exact measured basis.

For tokens, assisted input already includes the cue. Q is zero additional
agent tokens, justified by that accounting boundary, and estimated cue tokens
are diagnostic only. Input, cached-input, output, reasoning-output, and total
tokens remain separate; cached input is a subset of input, and reasoning
output is not added again to a total that already includes it. In this
deterministic no-LLM core, H has zero LLM tokens by construction, explicitly
labeled `derived`, while maintenance runtime and scan costs remain reported.
Do not convert tokens into currency without a separate pricing basis.

For execution duration, Q is the externally measured wall duration of one
successful shipped `repocue cue --view overview` invocation in the same
evaluation pair, from process launch through exit after stdout is fully
captured. It includes startup, repository/location-cache and SQLite access,
freshness inspection, serialization, cue-metric recording, and output. Supply
those captured bytes to the assisted agent; do not measure one invocation and
deliver a separately generated cue. Record executable digest, arguments,
cache/location-hint state, observation times, exit status, and output digest
in the external evidence. Require the expected snapshot, basis, schema,
budget, and `current` or `dirty-but-indexed` freshness before launching the
agent. Existing pre/post repository-state checks remain in force.

The in-process builder duration in existing evaluation-3 reports excludes
these CLI operations. Keep that value as a separately labeled diagnostic;
never substitute it for Q or add it again to a full CLI duration. When
full-path evidence is absent, Q is `not_observed` and combined duration
projections and their break-even values are unavailable. Agent-only duration
comparisons remain visible with that narrower boundary. This measurement
belongs to calibration evidence; routine reporting does not execute cue or
collect a new runtime-duration history.

H is the recorded local epoch baseline (or the rebaseline that created that
epoch) plus refresh durations from committed rows whose recording times are
before the cutoff. This is an accounting boundary, not proof of publication
before that instant. Use the full
recorded operation duration, not operation duration plus its already included
scan duration. Its unrecorded failed-operation and maintenance-process-startup
costs remain outside H and are disclosed. Do not charge a cue's freshness scan
again as refresh maintenance: it is already inside Q. The result is a serial
execution-duration projection of the stated components, not elapsed time for
concurrent consumers or a complete system-cost measurement. Task-ready latency
remains `not_observed`.

For command counts, distinguish agent-command projections from total Git
invocations. Agent-command comparisons exclude RepoCue internal operations
and identify that boundary. Total Git projections use recorded maintenance
Git invocations as H and observed Git invocations from the full cue CLI as Q,
charged once per consumer. Capture these invocations at the real process
boundary; do not substitute the builder's zero Git calls or a hard-coded count.
If observation is unavailable, Q is `not_observed`, not zero. Keep evaluation
setup and pre/post agent isolation checks outside Q and identify them as
experiment overhead. Do not combine generic tool calls, Git subprocesses,
and agent commands into one count. An unavailable component suppresses only
the affected total.

Do not charge calibration experiment baselines or agent runs as repeated
production maintenance. Display calibration execution costs separately. Live
H always covers the bound epoch from its creation, including a baseline that
predates `--window`; the window controls the observed activity/trend section
only. Record operation IDs and cutoff used for H. Missing epoch baseline
evidence at the cutoff makes combined runtime/Git projections unavailable,
even when the baseline's state observation precedes the cutoff. Do not combine
this epoch's H with a profile from another epoch.

For each pair with g_i > 0 and H >= 0, the first strictly cheaper positive
consumer count is `floor(H / g_i) + 1`. Thus equality at N is not break-even
under this definition, and zero maintenance with a favorable difference
starts at one consumer. For g_i <= 0, record `no_finite_break_even`; for missing
costs, record `not_observed`. Show per-pair results and finite-result coverage;
show a finite range only across finite results, without hiding non-finite
pairs. Never issue a cross-unit break-even result.

Historical operation duration, files/bytes scanned, Git commands, cue bytes,
estimated cue tokens, and counts use existing operation/cue rows. Compare
per-operation medians and counts as well as totals so more activity does not
automatically appear to be a performance regression. An unavailable prior
period has no percentage trend; a complete period with no recorded events has
zero recorded activity. Show observation coverage and do not assume cache
history existed before its creation.

Schema version 2 alone does not prove historical measurement coverage. The
version-1 migration assigned `DEFAULT 0` to `scan_duration_ms` and
`git_commands`, so a zero in either field without independent evidence that it
was measured is `not_observed`. Do not infer a measurement boundary from the
current executable, a guessed date, or the first nonzero row. Preserve raw
stored values; annotate their interpretation in report output. This rule is
specific to these ambiguous fields, not valid recorded zeros such as bytes
scanned by a no-op refresh or separately observed zero-cost measurements.

Expose known and unknown row counts per dimension. Known-only aggregates may
be shown with explicit partial coverage but cannot stand in for a full total,
percentage trend, H, or break-even result. Suppress only the totals affected
by unknown components; a missing Git count does not erase recorded duration.
No destructive history rewrite or new measurement-history table is required.

Current database size is a point measurement with its observation time.
Historical database-size trends and runtime cue-generation duration trends are
`not_observed` because the current schema does not retain them. Full cue CLI
duration from calibration remains a separate measured reference with its
recorded environment and cache state, not a live measurement for later
snapshots. M3 adds no history collection for these values. CPU, memory,
filesystem read/write bytes, freshness lag, and task-ready latency likewise
remain `not_observed` when
their observation path does not exist. Scan bytes and named-file-size proxies
are separate metrics and never produce a net I/O-savings calculation.

##### CLI And Periodic Execution Contract

The working implementation provides these commands; use a real imported profile ID.

```bash
repocue calibration import --reports /tmp/repocue-pairs --evidence /tmp/calibration-evidence.json --repository .
repocue report --repository .
repocue report --repository . --window 30d --format markdown
repocue report --repository . --profile calibration-example --consumers 5
```

Both commands accept `--repository PATH` (default current directory) and the
existing `--cache-dir PATH` override. Import prints the created calibration ID,
artifact path, digest, sample grade, and target binding as JSON. An unchanged
import is idempotent; it returns the same ID. Changing assessment content or
source evidence creates a distinct immutable profile. Derive the ID from
normalized evidence digests and the local binding; exclude import wall-clock
time from that identity. A later epoch needs a new verified binding and ID.

`report` defaults to JSON and 30 days. `--window Nd` accepts a positive integer
number of fixed 24-hour days, with checked arithmetic. Capture a single UTC
cutoff t at command start; use `[t - N days, t)` and the immediately preceding
equal interval. Add optional `--as-of <RFC3339-UTC>` for repeatable reports over
recorded observations, rejecting future cutoffs. Select the retained snapshot
with the latest `observed_at` strictly before the cutoff; break timestamp ties
by snapshot sequence. This means the state observed before the cutoff, not a
snapshot proven published before it. Use parsed UTC instants rather than
lexical comparisons of variable-width RFC3339 timestamps.

| Report selection | Stored time | Meaning |
| --- | --- | --- |
| Snapshot and epoch | Snapshot `observed_at`; epoch `created_at` / `superseded_at` | Initial state observation, not publication |
| Operation activity and H | Operation `created_at` | Metric-row recording time before transaction commit |
| Cue activity | Cue `created_at` | Cue-metric recording time, not confirmed agent receipt |
| Calibration eligibility | Binding observation time | Evidence acquisition, not import or publication completion |

Use `[start, cutoff)` for activity rows and strictly-before-cutoff selection
for bindings. No eligible snapshot yields unavailable historical state, not
the current snapshot as a substitute. If the chosen state precedes the cutoff
but its baseline metric does not, show the observed state and unavailable H.
Do not reconstruct publication time from duration or add publication history.

Current live checks and current database size are separate observations, never
historical facts. Historical applicability uses only retained binding/state
evidence available for the selected observation basis; absent evidence
suppresses projections, and today's fingerprint never fills a historical gap.
A later live change alone does not rewrite a historical observation. Profile
selection for a historical cutoff considers only bindings observed before it;
an explicitly selected later binding remains visible but cannot project.
Emit the cutoff, time meanings, profile digest, observation times, and stable
ordering; comparisons normalize report-generation time and explicitly current
live observations. Newly retained observations can change a repeated historical
report; reproducibility requires the same DB/evidence read basis as well as the
same cutoff, not an assumed historical copy of the database.

With no `--profile`, zero profiles produces maintenance data with unavailable
agent projections; one profile selects it and reports its applicability;
multiple profiles produces maintenance data and a `profile-selection-required`
warning listing IDs and model/benchmark labels, with no agent projection.
An explicit unknown profile ID, bad digest, invalid argument, or unreadable DB
is an error. A valid but inapplicable profile remains visible with projections
suppressed. Deleting the cache also removes its calibration copies; restore by
reimporting retained evidence, with the binding rules applied again.

The default consumer scenarios remain 1, 2, 3, 5, and 10. `--consumers N`
adds one explicitly requested positive-integer scenario, without duplicating
an existing value. Periods spanning epochs show activity per epoch and never
sum consumer projections across them. A selected profile can project only its
bound epoch, which must be active at the cutoff.

`report` does not refresh, record a cue run, persist a generated report, or
silently migrate an old database. It reads a coherent DB transaction and checks
live evidence without modifying source files. Schema version 2 is supported
read-only: report its available maintenance history with
no calibration profiles, so reporting works before the first import. Other
unsupported schemas fail explicitly. Concurrent changes yield
one coherent result or a defined retryable error. Follow existing CLI exit
conventions: success and informational unavailable results use exit 0; failures
use exit 1 with the existing JSON error format on stderr.

Scheduling documentation must specify absolute executable, repository, cache,
and output paths. Schedule report only; refresh remains an explicit operator
action. Show freshness and observation cutoff in every report. The caller
publishes stdout through a temporary file and rename only after exit 0 so an
error cannot replace a previous good report. Document external overlap
prevention for one destination. No daemon, timer installation, or automatic
artifact retention is performed by RepoCue.

##### Deterministic Git-Fact Correctness Contract

Validate and score each selected final response through the shipped
`internal/benchmark.ParseAndScore` path against its contemporaneous Git basis,
not a later repository state. Evidence must bind that basis to the report's
state fingerprint and digest and record the benchmark/answer schema, scorer
revision, and scoring time. Recompute rather than trust supplied score totals.
Keep the source basis and raw responses in external evidence; do not copy the
scorer's expected/actual values into calibration JSON, SQLite, or reports.

For each response, preserve report digest, pair ID, condition, run index,
scoring provenance, pass count, total fact count, ratio, and named per-fact
pass/fail results for branch, HEAD, dirty state, tracked changes, and untracked
paths. Calculated correctness is `derived` from the observed answer and
verified basis. A schema-valid wrong answer remains a scored failure, not an
import rejection or an excluded efficiency sample. Invalid answer shape or
contradictory supplied evidence remains an import error.

Display direct and basic ratios with medians, ranges, scored-response counts,
per-fact pass counts, and paired correctness differences calculated before
aggregation. Only complete scored pairs contribute to paired comparisons;
retain other valid individual scores and expose their coverage. Legacy input
that satisfies the repository-binding contract but lacks verifiable scoring
basis has `not_observed` correctness and a named limitation, not an inferred
pass. Show failed facts or incomplete correctness coverage alongside any
efficiency projection; do not claim equivalent quality from numeric savings.

These results concern Git facts at the measured basis only. They neither
establish semantic completeness nor become renewed quality measurements when
a profile is used for a later snapshot. Optional qualitative assessments
remain separate and their absence never suppresses available Git-fact scores.

##### Qualitative Assessment Contract

`--assessments <directory>` is optional. When supplied, the evidence file maps
each relative assessment path and SHA-256 digest to one selected report digest
and response ID. Validate the existing qualitative-assessment-2 schema and
match condition, run index, repository HEAD, and state fingerprint to that
report. Reject missing mapped files, ambiguous or duplicate response mappings,
mismatched basis, and mixed assessors within an aggregate. Require one selected
assessment per response; never average conflicting assessor judgments silently.

The importer preserves the six numeric scores, assessor, response ID, rubric
version, report/assessment digests, and coverage. Assessment time comes from
contemporaneous provenance in the evidence file because assessment-2 has no
timestamp field. If unavailable, report it as `not_observed`; import time and
file modification time are not assessment time. Measurement time and source
repository basis remain visible even when assessment time is unknown.

Aggregate each dimension only across complete assessed pairs, with median,
range, and assessed-pair count. No assessments yields `not_observed` semantic
quality and `quality-not-assessed`; partial coverage is explicit. This does not
replace the separate deterministic Git-fact results. Numeric efficiency
projections can still be shown, but they do not establish maintained context
quality. Do not modify scores or persist free-text assessment evidence, agent
responses, or prompts in calibration JSON or SQLite. Keep those externally
under their digests for independent inspection. Runtime Git freshness is a
separate current observation, not a renewed semantic assessment.

##### Dependencies And Decisions

- D1-D23 define the accepted reporting and storage boundaries.
- M1 has no dependency on G1; implementation and local fixture verification can
  finish before real calibration evidence is authorized.
- The report must follow the evaluation requirements in
  `docs/PROJECT_BRIEF.md` and the evidence boundary in `docs/DATA_MODEL.md`.

##### Implementation Plan

Plan Status: accepted
Plan Acceptance: 2026-09-07
Implementation Authorization: User authorized the current M1 implementation on 2026-09-07; no real-agent calibration or Git publication.
Superseded Plan Artifacts: none

1. Add an ADR defining the calibration artifact boundary, SQLite reference
   boundary, applicability states, per-dimension accounting, and absence of an
   aggregate cross-unit cost score. Include content-sensitive evidence scope,
   ambiguous legacy zero handling, and observation-based historical reporting.
2. Add JSON Schemas for `repocue/calibration-1`,
   `repocue/calibration-evidence-1`, and `repocue/savings-report-1`, including
   explicit measurement status, unit, basis, limitations, source-file joins,
   full CLI cue-cost observations, deterministic Git-fact results, and nullable
   values for `not_observed` measurements. Extend the evaluation preparation
   path to emit evidence and allow direct/basic-only execution. Measure the
   shipped cue executable and deliver its captured stdout; preserve default
   five-condition behavior and old report schemas. Verify through T13 and T18.
   Add the shared content-sensitive capture at evaluation, import, and report
   boundaries, including pre/post agent checks; retain legacy fingerprints as
   separate provenance. Verify same-path content changes through T20.
3. Add a calibration importer that validates every source report and its shared
   repository, state, runner, model, reasoning, benchmark, output-schema,
   cue-schema, cue-view, cue-budget, RepoCue-version, selection-policy, tracked
   file count, indexed byte count, and language-profile metadata before
   calculating paired medians, ranges, and sample counts. Implement the exact
   source-to-local binding, deterministic scoring through the existing
   benchmark package, and optional assessment joins defined above. Reject
   legacy evidence that cannot supply verified required provenance; retain
   missing optional observations explicitly. Verify correctness and privacy
   through T9 and T19.
4. Write calibration JSON atomically below the repository's RepoCue cache
   directory. Publish its SQLite reference only after validating the final
   file digest; reject missing or changed referenced artifacts when reporting.
5. Migrate SQLite from schema version 2 to version 3 with only calibration
   reference and applicability metadata. Preserve transactional rollback and
   do not store raw reports or agent responses.
6. Implement one calculation model shared by JSON and Markdown. Keep token,
   command, execution-duration, scan, storage, and proxy-byte dimensions
   separate; never add unlike units into one total.
7. Implement exact-basis, compatible-snapshot, and incompatible applicability.
   A change in epoch, repository identity, RepoCue contract, cue contract,
   agent/model configuration, benchmark, or output schema makes the profile
   inapplicable. A compatible snapshot remains an estimate and reports its
   tracked-file, indexed-byte, and language-profile drift without inventing an
   unsupported validity threshold. Distinguish core freshness from a changed
   untracked-content binding under the same snapshot; suppress that profile's
   projections without promising that ordinary refresh repairs it. Verify T20.
8. Implement epoch-segmented time windows and equal-window trends for the
   existing baseline, rebaseline, refresh, files-scanned, bytes-scanned,
   Git-command, cue-count, cue-byte, and cue-token-estimate history. Report
   current database size separately and mark unavailable historical size and
   runtime cue-duration trends `not_observed`. Interpret ambiguous legacy zeros
   per dimension without rewriting stored rows or inventing measured values;
   verify coverage and suppressed totals through T21. Use the explicit time
   mappings for state, activity, H, and binding selection; verify T22.
9. Implement fixed reuse projections and explicit consumer scenarios. Show
   baseline and refresh as one-time maintenance, assisted agent totals as
   per-consumer cost plus full cue CLI cost where comparable, and negative
   results without suppression. Show deterministic correctness alongside the
   costs. Calculate break-even separately for each comparable dimension and
   handle zero, unfavorable, or unobserved per-consumer differences without
   division. Verify full-path accounting and missing costs through T15 and T18.
10. Emit required but unavailable CPU, memory, filesystem-I/O, freshness-lag,
    and task-ready-latency measurements as `not_observed` with the missing
    observation named. Keep named-file-size proxies as a separately labeled
    diagnostic value.
11. Add CLI and operator documentation, including calibration provenance,
    stale-evidence warnings, stdout-only report handling, and external
    scheduling examples using the CLI contract above. Update
    `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`, `docs/EVALUATION.md`, and
    `docs/CLI.md` only when the described behavior is implemented. Run a
    mandatory second-person documentation pass before commit.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Contract | Validate valid, incomplete, cross-repository, incorrectly paired conditions, mixed-model, mixed-benchmark, mixed-budget, and negative-value report sets through the shipped importer | Go test with versioned evaluation fixtures | Only a complete compatible direct/basic set produces `repocue/calibration-1` |
| T2 | Calculation | Run table-driven projection and break-even cases for 0, 1, 2, 3, 5, 10, and explicit N consumers, including zero denominators, negative savings, and unavailable components | Go unit test | Per-dimension arithmetic is exact; cue tokens are not double-counted; zero and unfavorable cases have no break-even value; no cross-unit total is produced |
| T3 | Storage | Open real version 2 databases, migrate through the shipped storage package, force failures before and after artifact publication, and reopen | Temporary filesystem and real SQLite driver | Migration reaches version 3 atomically; failures expose neither a partial reference nor a trusted digest |
| T4 | Integration | Run init, refresh, cue, calibration import, and report through the shipped CLI against a temporary real Git repository | Linux temporary repository | JSON report is current, schema-valid, deterministic, and leaves repository Git state unchanged |
| T5 | Applicability | Change snapshot, cue budget, benchmark metadata, and epoch through real commands and report again | Linux temporary repository | States are respectively compatible estimate or inapplicable as defined; no inapplicable projection is emitted |
| T6 | Epoch window | Create operation history on both sides of a real rebaseline and request a spanning window | Linux temporary repository | Results are separated by epoch and no combined projection crosses the boundary |
| T7 | Missing observations | Report with no calibration and with unavailable CPU, memory, filesystem-I/O, freshness-lag, and task-ready fields | Go and CLI tests | Every missing value is `not_observed`; none is absent or zero-filled |
| T8 | Rendering | Generate JSON and Markdown from the same shipped report calculation and compare normalized values | Go test | Both formats contain identical measurements, provenance, applicability, and limitations in deterministic order |
| T9 | Privacy | Import fixtures containing final responses, prompts, and absolute paths, then inspect calibration, SQLite, JSON, and Markdown outputs | Go integration test | No excluded raw content or absolute evaluated-repository path is persisted or emitted |
| T10 | Concurrency | Run report reads while real refresh and calibration-import transactions contend | Go integration test with real SQLite | Readers see one coherent reference and snapshot or a defined retryable error, never partial current state |
| T11 | Distribution | Run the full Go suite, race detector, vet, module-tidiness check, CGo-free Linux static build, and Windows amd64 cross-build | Supported build environment | All checks pass without adding CGo or an external runtime |
| T12 | Binding | Import real evaluation evidence into a different temporary DB for the same repository; repeat with colliding snapshot IDs, changed index, untracked state, and concurrent rebaseline | Shipped evaluation/import CLI and temporary real Git repositories | Source IDs are provenance only; matching fingerprint binds the local epoch; mismatches and races publish no reference |
| T13 | Evidence compatibility | Execute direct/basic preparation without an agent for evidence-path checks, retain the default five-condition check, and import versioned observed-run fixtures with missing producer fields, bad digests, duplicate pairs, and wrong conditions | Real evaluation path plus external-runner fixtures | Evidence is emitted from the real preparation path; dry runs cannot calibrate tokens; only complete verified observed input imports |
| T14 | History coverage | Request size and cue-duration trends with a version 2 history migrated through import, including a period predating cache creation | Real SQLite and report CLI | Point size remains dated; unsupported trends are `not_observed`; absent history is not zero activity |
| T15 | Pair accounting | Recalculate an asymmetric three-pair fixture where median differences and difference of medians disagree; include a pre-window baseline, full cue CLI duration and Git counts, missing or builder-only cue costs, zero costs, and equality at break-even | Shipped report calculation | Paired formulas and full epoch H agree with explicit expected arithmetic; missing full-path costs suppress only affected totals; calibration maintenance and cue freshness work are not charged twice |
| T16 | CLI contract | Exercise defaults, UTC boundary timestamps, as-of, profile counts 0/1/2, missing profile, stale cache, invalid ranges, explicit consumer count, and output failure | Shipped CLI and real temporary DB | Selection, windows, exit codes, suppressed projections, and read-only behavior match the contract; the prior published report survives caller failure |
| T17 | Assessment import | Import no, partial, and complete assessment-2 sets; test missing dates, digest and response joins, duplicate mappings, wrong HEAD, and mixed assessors | Shipped importer and versioned external assessment fixtures | Scores are unchanged; coverage and unknown dates are explicit; invalid joins fail; free-text evidence is excluded |
| T18 | Full cue path | Prepare direct/basic evidence through the shipped evaluator and cue executable; observe real process exit, stdout digest, and Git subprocess events; exercise cold and warm location hints, dirty-but-indexed state, and cue failure | Real binaries and temporary Git repositories; replace only the external agent boundary with a fixture runner | Evidence covers the complete CLI and delivers the same stdout to the runner; snapshot and freshness match; failed cue execution launches no agent; no real Codex run is required |
| T19 | Deterministic correctness | Import versioned answers with each of the five Git facts wrong separately, all-correct answers, missing legacy scoring basis, tampered score totals, and partial pair coverage; render JSON and Markdown without qualitative assessments | Shipped importer, benchmark scorer, and report CLI; temporary Git repositories and external evidence fixtures | Independent expected counts match recomputed scores; wrong valid answers remain in efficiency samples; missing evidence stays unavailable; both formats retain per-fact outcomes and coverage without raw expected/actual values |
| T20 | Untracked-content evidence | Change only an existing untracked file's content between real captures and during a fixture runner; repeat with unchanged names/status, symlink target text, unreadable entries, legacy names-only evidence, import, and report | Shipped evaluation/import/report paths and temporary real Git repositories; substitute only the external agent boundary | Strong fingerprints differ when content differs; incomplete evidence never matches; persistent runner changes publish no set; import rejects mismatches; report suppresses same-snapshot mismatches even after ordinary refresh; raw content stays external; the content-change regression fails with the old capture |
| T21 | Legacy metric coverage | Run the shipped migration on a populated version-1 DB fixture, then report before and after calibration import; include native version-2 rows, legitimate zero scan bytes, and mixed known/unknown metrics | Real SQLite migration and report CLI with versioned database fixtures | Ambiguous scan-duration/Git zeros remain unavailable; raw rows are preserved; coverage is per dimension; affected totals/trends/H/break-even are suppressed without discarding other observations |
| T22 | Observation-time history | Run init, refresh, rebaseline, cue, and binding operations; select cutoffs at and between their recorded observation and metric times; repeat historical reports after later live changes and bindings | Shipped CLI, temporary real Git repositories, and real SQLite; controlled clock only at the outer boundary if required | Strict cutoff and tie rules hold; a state can precede its baseline metric without inventing H; absent historical evidence stays unavailable; later live state or bindings do not replace earlier evidence; no publication-time claim is emitted |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | 2026-09-07 | Linux; real CLI and external-input fixtures | Passed | Incompatible-input and three-pair tests; `TestReportingEstimatedAgentCostsStayDiagnostic`; `TestReportingCueObservationMustPrecedeAssistedRun` |
| T2 | 2026-09-07 | Go calculation tests | Passed | `TestPairwiseMedianAndProjectedCosts`; `TestProjectionUnavailableAndStrictBreakEven`; `TestConsumerScenariosAndArithmeticSafety` |
| T3 | 2026-09-07 | Real SQLite; temporary filesystem | Passed | `TestVersionTwoReadOnlyAndVersionOnePopulatedMigration`; `TestCalibrationPublicationFailuresAndReopen` includes deferred-FK COMMIT failure after file publication |
| T4 | 2026-09-07 | Linux; built CLI and real Git | Passed | `TestReportingCLIImportsRealEvidenceAndPreservesRepository` |
| T5 | 2026-09-07 | Linux; real CLI | Passed | `TestReportingCLIStateTransitionsHistoricalCutoffAndInvalidFlags`; `TestReportingChangedSnapshotAndProducerContract`; mixed budget/benchmark are rejected through import |
| T6 | 2026-09-07 | Linux; real rebaseline and report | Passed | `TestReportingCLIStateTransitionsHistoricalCutoffAndInvalidFlags` retains three separate epoch activity groups |
| T7 | 2026-09-07 | Go and built CLI | Passed | `TestReportingCLIImportsRealEvidenceAndPreservesRepository`; no-profile and missing-cost cases |
| T8 | 2026-09-07 | Shared calculation; JSON and Markdown CLI | Passed | `TestReportingCLIImportsRealEvidenceAndPreservesRepository` compares normalized output values |
| T9 | 2026-09-07 | CLI, SQLite, calibration JSON | Passed | `TestReportingVersionTwoReadOnlyAndPrivateCalibrationStorage`; excluded response/path markers remain external |
| T10 | 2026-09-07 | Concurrent processes and real SQLite | Passed | `TestReportingCLIConcurrentReadersRefreshAndImport`; `TestReportingChangedSnapshotAndProducerContract`; `TestHistoryReadersRemainCoherentDuringPublicationAndRefresh` |
| T11 | 2026-09-07 | Go 1.26.7; Linux amd64 | Passed | Full suite and race suite, vet, tidy -diff, static Linux build, Windows amd64 cross-build; commands below |
| T12 | 2026-09-07 | Evaluation/import CLI and real SQLite | Passed | Source/local ID divergence, another checkout, changed index, untracked mismatch; `TestCalibrationPublicationRechecksCurrentStateAndDigest` schedules real rebaseline before stale-state publication |
| T13 | 2026-09-07 | Real preparation and external-runner fixtures | Passed | `TestDirectBasicEvidenceRunsShippedCueWithoutOracleOrAgent`; existing M2 suite retains five-condition coverage; incompatible-input tests reject missing producer, digest, pairing, and condition evidence |
| T14 | 2026-09-07 | Real SQLite and report CLI | Passed | `TestReportingLegacyDBCoverageBeforeAndAfterImport`; `TestReportingVersionTwoReadOnlyAndPrivateCalibrationStorage`; pre-cache coverage tests |
| T15 | 2026-09-07 | Shipped calculation and CLI | Passed | Asymmetric paired median 10 versus difference of medians 110; full-Q and maintenance tests; `TestEstimatedAgentMeasurementsAreExcludedFromArithmetic` and actual CLI diagnostic-only source estimates |
| T16 | 2026-09-07 | Linux CLI; temporary cache/output | Passed | State/flag/profile/cutoff tests; real `/dev/full` failure; `TestReportingDocumentedPublisherPreservesLastSuccess` executes the documented wrapper |
| T17 | 2026-09-07 | Shipped importer; external assessment fixtures | Passed | `TestReportingOptionalAssessmentsHaveExactJoinsAndCoverage` |
| T18 | 2026-09-07 | Built evaluator/cue; synthetic agent boundary | Passed | Cold/warm/dirty, failure-before-agent, delivered-stdout digest tests; real CLI rejects post-agent and overlapping cue intervals in `TestReportingCueObservationMustPrecedeAssistedRun` |
| T19 | 2026-09-07 | CLI, real benchmark scorer, external answers | Passed | `TestReportingCorrectnessRetainsWrongAnswersAndMissingCoverage`; wrong fact gives 4/5, not sample removal; tampered scoring rejected |
| T20 | 2026-09-07 | Real Git capture/evaluation/import/report | Passed | Content, symlink, unreadable-entry and cancellation tests; old names-only capture stays unchanged while strong capture differs; runner mutation rejects publication; ordinary refresh does not repair mismatched binding |
| T21 | 2026-09-07 | Real migrated v1 and native v2 SQLite input | Passed | Populated migration and legacy CLI tests preserve rows; ambiguous zeros remain unavailable; legitimate no-op bytes remain observed |
| T22 | 2026-09-07 | CLI and real SQLite observation times | Passed | `TestHistoryObservationCutoffsAndSequenceTies`; `TestReportingCutoffBeforeMetricAndOutputFailure`; historical CLI state remains unchanged after later live changes |

Verification commands use `GOCACHE=/tmp/repocue-m3-go-cache` on this host:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod tidy -diff
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/repocue-m3-linux ./cmd/repocue
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/repocue-m3-windows.exe ./cmd/repocue
make docs.build
git diff --check
gofmt -l cmd internal docs/schema
```

Named CLI tests are in `internal/app/reporting_test.go`; storage tests are in
`internal/storage/reporting_test.go`; calculation/history tests are in
`internal/reporting/`; cue process tests are in
`internal/evaluation/calibration_test.go`; content tests are in
`internal/repository/evidence_test.go`. The external agent is synthetic. These
results verify software behavior, not actual agent savings. Full Windows
runtime execution, hardware power loss, and installation of cron/systemd
schedules were not performed. Builds preserve `go.mod` and `go.sum` unchanged.

Independent storage/evidence/history and evaluation/calculation/CLI reviews
accepted the working implementation on 2026-09-07, including the required
second-person documentation checks. Independent closure acceptance followed
on 2026-09-08 UTC. The accepted implementation is committed and matches the
fetched upstream. No new real-agent calibration was performed; M1 acceptance
establishes the reporting path, not real Codex measurement coverage or savings.

##### Closure Evidence

- Complete: 2026-09-08 UTC. Implementation and required T1-T22 checks are
  recorded above; no linked GitHub issue or unresolved M1 external gate exists.
- Commit: `a8f650cc8950015b22a7873001ef72234ea3cf3c`,
  `Add evidence-based savings reports`; 34 implementation, contract, and
  documentation files. The separately authorized commit and push completed.
- Landing observation: 2026-09-08 00:31:16 UTC, `origin/master` fetched;
  local HEAD and upstream both resolve to that commit. The working tree was
  clean and the full tracked-file comparison against `origin/master` had no
  differences. Recheck with `git fetch origin`, `git status --short --branch`,
  `git rev-parse HEAD origin/master`, and `git diff --exit-code origin/master`.
- The closure cross-check accepted the 34-file candidate and its status
  derivatives. The later schema-only EOF correction was verified with the
  shipped schema tests and diff check before publication; it did not change
  schema values. The broader verification above remains dated 2026-09-07.
- This closes the implementation work unit, not the local review-session
  directory. Session removal and new Git publication are not authorized.

##### GitHub Projection

Title: Add quantitative savings reports
Labels: none
GitHub Milestone: none
Observed State: none
Observed Labels: none
Observed Milestone: none
Last Compared: never

#### M2 - Initial Repository Calibration And Report

Origin: 10a4f02 / M2
Identity History: none
GitHub Issue: none
Status: Blocked

##### Summary

Produce the first repository-specific calibration and periodic report only
after the reporting path is complete and the User authorizes the evidence
source. A single direct/basic pair verifies the pipeline but remains
exploratory. Reported ranges require at least three controlled pairs per arm.
The initial target is the RepoCue repository itself at the exact basis
recorded in G1. Results are repository-specific and do not establish general
cross-repository effectiveness.

##### Scope

- Validate an authorized retained report set or run fresh direct/basic pairs.
- Use the same repository state, Codex version, model, reasoning, benchmark,
  output schema, sandbox, cue view, and cue budget for every pair.
- Produce at least three complete pairs for the initial calibration.
- Generate and independently re-derive the first report and its fixed reuse
  projections.

Out of scope: additional experimental conditions, optimization based on the
result, or selection of a structural backend.

##### Completion Criteria

- At least three valid direct/basic pairs exist for one exact repository basis.
- Before and after content-sensitive fingerprints show matching Git, tracked,
  and non-ignored untracked state for every agent run, including untracked
  content. They do not claim coverage of ignored files or intermediate writes.
- The calibration records median, range, sample count, exact run metadata, and
  source-report-set digest.
- The source set includes calibration-evidence-1 with verified producer
  provenance, paired report digests, full cue CLI duration and Git observations,
  deterministic Git-fact results with scoring basis, and any optional assessment
  joins. Retained sets missing these observations do not complete M2; renewing
  evidence remains subject to G1 authorization.
- The first report distinguishes observed local maintenance, measured
  reference values, compatible estimates, unavailable metrics, and explicit
  consumer projections.
- An independent calculation reproduces every projected token, command, and
  execution-duration value.
- Independent scoring reproduces the five Git-fact outcomes for every
  response, including failures; quality and efficiency results use the same
  selected responses with explicit coverage.
- No significance claim is made from three runs.

##### Dependencies And Decisions

- M1 and the M3 token correction are Complete in `a8f650c` and `8d61f6e`.
- G1 must be Complete; resume as Not started after all dependencies complete.
- If retained source reports cannot be verified, G1 requires a separate cost
  estimate and User authorization for new Codex runs.

##### Implementation Plan

Plan Status: draft
Plan Acceptance: none
Implementation Authorization: none
Superseded Plan Artifacts: none

1. Resolve G1 with the exact report source, repository basis, run count, model,
   reasoning, sandbox, benchmark, cue budget, and estimated run cost.
2. Validate retained reports or run the authorized direct/basic pairs without
   modifying the evaluated repository. Use the explicit two-condition path
   and retain its evidence JSON, full CLI cue costs, and contemporaneous
   scoring basis with content-sensitive pre/post evidence; six agent runs form
   three complete pairs. Do not replace missing historical observations with
   current CLI timings, untracked content, or Git state.
3. Import the complete report set with its evidence file, bind it to the same
   repository's current local epoch, and record the calibration artifact
   digest. Include authorized assessments when available; otherwise report
   semantic quality as `not_observed`.
4. Generate JSON and Markdown reports for 1, 2, 3, 5, and 10 consumers.
5. Recompute all derived values independently from the retained reports and
   evidence, including full-path cue accounting and deterministic correctness,
   and compare them with the generated report.
6. Record findings without claiming general significance or actual unobserved
   consumer savings.

##### Measurement Preflight

Observed on 2026-09-08 UTC at implementation commit `a8f650c`:

- The repository contains 101 tracked files and 837,608 Git blob bytes, derived
  from `git ls-tree -rl HEAD`. This is a corpus profile, not measured scan I/O.
- The local CLI reports `codex-cli 0.153.4`; `codex login status` reports
  ChatGPT authentication. No model availability probe or agent run occurred.
- No retained real direct/basic report set was found in the repository or its
  local `work/` artifacts. Synthetic integration fixtures are not calibration.
- The Codex adapter at `a8f650c` loses missing usage fields. An isolated Go test
  overlay reused `TestRunInvokesCodexAtOutermostBoundary`, omitting only
  `reasoning_output_tokens` from the external Codex fixture's usage event and
  adding a missing-value assertion. The unmodified shipped `Run` path emitted
  `reasoning_output_tokens: 0` with status `observed`; the assertion failed.
  The input, cached input, and output values remained 10, 0, and 5. Only the
  external executable boundary was substituted; production code was unchanged.
  Evidence locations: `internal/codexadapter/adapter_test.go` and
  `internal/codexadapter/adapter.go` (`codexUsage`, `buildObservation`).
- The adapter observation conflicts with D2. Its separately authorized
  correction and missing-versus-zero verification belong to M3 below.
  This is not a claim that Codex 0.153.4 always omits that field.
- Routine local refresh encountered a read-only cache database in the current
  sandbox. Cache write access must be established before local calibration
  binding; no cache initialization or permission change was performed.

These preparation checks do not complete M2 / T1-T4. The owner authorized the
bounded M3 token correction separately; G1 still excludes new Codex runs.

The correction subsequently landed in `8d61f6e` on 2026-09-08 UTC; M3's
completion evidence is recorded below. That resolves the token-correction
prerequisite, not G1 authorization or the writable-cache preflight.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Evidence validation | Validate every source report and the complete paired set through the shipped importer | Authorized retained or fresh report set | All reports share the pinned conditions and pass their versioned schemas |
| T2 | Isolation | Compare versioned content-sensitive evidence before and after every real agent run, including untracked regular-file content and symlink target text | Evaluated Git repository | Branch, HEAD, index, tracked working tree, and non-ignored untracked state match before and after each run; names-only legacy fingerprints cannot satisfy this check |
| T3 | Recalculation | Recompute medians, ranges, sample counts, full CLI cue-cost projections, and five Git-fact scores from source reports and contemporaneous evidence with an independent deterministic tool | Retained reports and generated calibration | Every generated value matches the independent calculation; wrong answers remain represented; missing observations are not zero-filled |
| T4 | Presentation | Validate JSON against schema and compare Markdown to normalized JSON | Generated first report | Values, classifications, provenance, and limitations agree |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | Not run | Authorized report set | Pending | none |
| T2 | Not run | Evaluated Git repository | Pending | none |
| T3 | Not run | Independent deterministic calculation | Pending | none |
| T4 | Not run | Generated report | Pending | none |

##### Closure Evidence

- none

##### GitHub Projection

Title: Calibrate RepoCue and publish the first savings report
Labels: none
GitHub Milestone: none
Observed State: none
Observed Labels: none
Observed Milestone: none
Last Compared: never

#### M3 - Token Usage Absence And Report Compatibility

Origin: 10a4f02 / M3
Identity History: none
GitHub Issue: none
Status: Complete

##### Summary

Preserve missing Codex token observations without erasing available costs or
correctness. Introduce evaluation and runner version 4 while retaining strict
read compatibility for unchanged version-3 reports.

##### Scope

Optional per-turn counters, complete-turn aggregation per counter, checked
arithmetic, adapter provenance, matching version dispatch, local regression
tests, and current contract/status documentation.

Out of scope: real model runs, calibration acceptance, SQLite changes, new
dependencies, performance optimization, and changes to existing v3 schemas
or retained report bytes.

##### Completion Criteria

- Missing/null counters stay unavailable and explicit zero stays observed.
- A completed turn without usage is retained; partial coverage never yields
  a complete aggregate. Input/output alone determine total tokens.
- Invalid counters, overflow, unknown/mixed versions, and malformed legacy
  input fail explicitly. Unrelated measurements and correctness survive.
- The real adapter/evaluator/import/report path and actual pre-change v3
  report bytes pass the checks below, with independent review.
- Required checks and the reviewed correction have Git landing evidence.

##### Dependencies And Decisions

M1 supplies the reporting path. D2, D14, and D24 define measurement presence
and version compatibility. G1 is not needed for local fixture verification;
it remains required for real calibration. The separately authorized commit
and push completed on 2026-09-08 UTC.

##### Implementation Plan

Plan Status: accepted
Plan Acceptance: 2026-09-08; bounded independent evaluation preflight accepted
Implementation Authorization: 2026-09-08; User authorized missing-token correction, new JSON versions, and existing-report read compatibility
Superseded Plan Artifacts: none

1. Retain actual pre-change v3 output before editing the implementation.
2. Update `internal/codexadapter/adapter.go`, `internal/evaluation/m2.go`,
   and `internal/evaluation/usage.go` for optional counters, complete-turn
   aggregation, checked arithmetic, and v4 execution.
3. Add `docs/schema/evaluation-v4.schema.json` and
   `docs/schema/evaluation-runner-v4.schema.json`; dispatch imports in
   `internal/evaluation/calibration.go` and
   `internal/reporting/calibration.go` without weakening v3.
4. Exercise the shipped paths in adapter, evaluation, schema, and CLI tests.
   Keep the actual legacy output under `internal/evaluation/testdata/`.
5. Update `docs/EVALUATION.md`, this detail, and `docs/CURRENT_STATUS.md`;
   independently verify the bounded correction before Git publication.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Usage | Run table-driven arithmetic/schema checks and shipped adapter with an external Codex executable fixture | Go; Linux | Missing/null/zero, partial turns, absent usage, invalid values, and overflow follow D24 |
| T2 | Pipeline and compatibility | Run actual CLI/adapter/evaluator/import/report; import unchanged output generated by the pre-change CLI | Real Git and SQLite; only external Codex substituted | Five presence cases preserve unrelated metrics and JSON/Markdown agreement; old digests and declarations survive; invalid version mixes fail |
| T3 | Regression and distribution | Full tests, race, vet, tidy -diff, static Linux build, Windows cross-build | Go; Linux amd64 host | All checks pass without dependency or default CGo changes |
| T4 | Contracts and presentation | Check frozen v3 digests, gofmt, ASCII, diff, mdBook, and independent second/third-person verification | Source and generated book | No v3 byte changes, invalid docs, or unsupported claims |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | 2026-09-08 UTC | Linux; actual adapter executable boundary | Passed | `TestRunPreservesUsagePresence`, `TestRunRejectsInvalidUsageAtOutermostBoundary`, usage/schema tables |
| T2 | 2026-09-08 UTC | Real CLI/Git/SQLite; synthetic external Codex | Passed | `TestReportingCodexUsageThroughShippedPipeline` covers five cases; incompatible-input tests; original v3 import and both report formats exited 0 |
| T3 | 2026-09-08 UTC | Go; Linux amd64 | Passed | Full tests, race, vet, tidy -diff, CGo-free static Linux build, and Windows amd64 cross-build exited 0; dependencies unchanged |
| T4 | 2026-09-08 UTC | Source and mdBook | Passed | v3 schema and golden digests match; gofmt, ASCII, diff, and mdBook passed; independent code/legacy review and final second-person review accepted with no remaining finding |

The golden v3 report was generated through the real `prepareReportingFixture`
path at `a8f650c`, not by relabeling current output. Its SHA-256 is
`681b676c0a0d12cb14237ed530fd9ccf32bc9353867affd7f0a973fe78b1c463`.
The new CLI imported the unchanged original direct/basic set and retained
its declared zeros. Producer incompatibility correctly suppressed projections
for that old executable; read compatibility does not waive applicability.
No actual agent token savings or Codex 0.153.4 usage coverage was measured.

##### Closure Evidence

- Complete: 2026-09-08 UTC. Required T1-T4 results and independent correction,
  second-person, and closure acceptance are recorded; Git publication is
  complete. M1 remains Complete.
- Commit: `8d61f6e65ab01ebbaef90ea2e6352b87235e195d`,
  `Preserve missing token measurements across evaluation reports`; 17
  implementation, test, contract, and documentation files. The separately
  authorized commit and non-forced push completed.
- Publication rechecked on 2026-09-08 UTC: local `HEAD`, `origin/master`, and
  GitHub `refs/heads/master` resolve to that commit. The working tree was clean
  before this status-only update. Recheck with `git rev-parse HEAD origin/master`,
  `git ls-remote --exit-code origin refs/heads/master`, and `git status --short --branch`.
- Product verification above remains dated 2026-09-08; it is not a new test run
  for this status update. No real calibration has occurred; M2 remains Blocked
  on G1. Whole-session closure, removal, and new Git publication are not
  authorized by this status update.

#### G1 - Calibration Evidence Authorization

Origin: 10a4f02 / G1
GitHub Issue: none
Status: Open

##### Summary

The User selects and authorizes the evidence source for M2. The source may be a
retained raw direct/basic report set that passes current validation or newly
run Codex pairs with an explicit cost estimate and pinned execution conditions.
Legacy reports without sufficient contemporaneous provenance cannot satisfy
this gate solely because their original report schema validates.

##### Draft Execution Conditions

Prepared: 2026-09-08 UTC. These are proposed settings, not execution authority.
The initial corpus is RepoCue only; no other local repository is included.

| Setting | Proposed value or required observation |
| --- | --- |
| Repository | The current RepoCue checkout; all six runs and local binding use the same root |
| Basis | A clean master commit containing accepted M1 and any separately accepted adapter correction; exact full SHA and content-sensitive fingerprint must be recorded before approval |
| Reference implementation | `8d61f6e65ab01ebbaef90ea2e6352b87235e195d`; contains accepted M1 and M3; the exact measurement basis still requires G1 approval |
| Runs | Three direct/basic pairs, six fresh ephemeral executions, one pair at a time |
| Codex | Pin the observed `0.153.4`; recheck before execution and stop on drift |
| Model and reasoning | `gpt-5.6-luna`, `medium`; no model substitution or automatic retry |
| Task and answer | `docs/benchmarks/repository-state-v2.md` and `docs/schema/benchmark-answer-v2.schema.json`; version 2 |
| Permissions | `read-only`; the shipped adapter uses `--ephemeral`, `--ignore-user-config`, `--ignore-rules`, and JSONL output |
| Assisted context | Basic overview only, maximum 500 estimated cue tokens; normal fallback discovery remains allowed |
| Execution path | Shipped `evaluate-m2 --conditions direct,basic`; not the five-condition shell wrapper |
| Evidence location | Caller-owned directory outside the evaluated checkout; record its exact path at approval; never stage raw responses |
| Local state | Same-checkout, current-epoch binding after validated import; writable cache access required |

The current evaluator requires direct then basic within each pair. This draft
keeps that order and reports the order/cache bias; it does not claim randomized
assignment. Both arms inherit the same available instructions and environment;
`--ignore-rules` concerns execution-policy rules, not a guarantee that repository
instruction files are absent. Audit direct-arm commands for unintended RepoCue
use before treating a pair as direct discovery.

Stop after the first pair to report observed usage, duration, deterministic
correctness, and any failure; obtain confirmation before the remaining two
pairs. A failed or interrupted pair is incomplete and does not authorize a
replacement run. Stop on state drift without reset or clean. Record attempts
and exclusions instead of selecting only favorable or correct responses.

##### Cost Preparation

No same-repository real-run evidence is available to forecast token consumption
or elapsed time reliably. The 500-token cue budget limits only the cue, not
the agent's total input, output, or execution time. A numerical time estimate
and a hard spending ceiling are unavailable; the current adapter has no
built-in deadline or billing cap. The first-pair checkpoint bounds the number
of initially started runs, not their token cost.

Rates retrieved on 2026-09-08 UTC: GPT-5.6 Luna uses 5 credits per million
uncached input tokens, 0.5 per million cached input tokens, and 30 per million
output tokens [1]. ChatGPT authentication does not establish the account's
remaining allowance, credit balance, or extra charge; those are not observed.

For an explicitly assumed six-run total of 1.5 million uncached input tokens
and 120,000 output tokens, the base-rate illustration is **11.1 credits**.
It is not a measured prediction, approved budget, or maximum. Recalculate from
the first pair before proposing the remaining four executions.

For comparison only, the API base rates are USD 0.20 / 0.02 / 1.20 per million
input / cached input / output tokens [2]. The same assumed uncached scenario
is USD 0.444 before adjustments, not an estimate of this ChatGPT account's
bill. Requests over 272,000 input tokens have 2x input and 1.5x output rates;
cache writes use 1.25x uncached input rates [2]. Aggregate run usage alone
cannot reconstruct these request-level charges.

##### Completion Criteria

- The evidence source, exact conditions, run count, repository basis, and any
  external execution cost are recorded with User authorization.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| Not run | Pending | User authorization and validated report-source record |

##### Closure Evidence

- none

#### M4 - Task-Specific Context Delivery And Local Measurement Readiness

Origin: 10a4f02 / M4
Identity History: none
GitHub Issue: none
Status: Complete

##### Summary

Provide a small task-specific context cue for repository instructions, design
decisions, and verification procedures. Explicit task and path matching selects
author-declared source references. Deterministic extraction delivers the chosen
original text with its source identity, current content digest, snapshot, and
freshness. The source remains authoritative; a cue does not grant permission
or establish that an agent understood or followed it.

Local measurement readiness adds the evidence contract and real-path checks
needed for M5. Existing command classification is insufficient to establish
successful source delivery or verification execution because it does not
retain command exit status or output in the runner observation.

##### Scope

- A versioned JSON declaration records item ID, kind, explicit task/path
  applicability, required status, original-source reference, and completion
  check reference. Sources belong to the evaluated Git repository.
- A separately versioned task cue selects applicable items and provides
  original-source text and provenance without automatic semantic summaries.
- Define source selection, digest checking, stable output ordering, missing
  inputs, conflict/ambiguity reporting, and required-versus-optional budgeting.
- Add explicit CLI task and target-path inputs, schema validation, source
  extraction, freshness checks, and measurements for the complete shipped cue
  path. Keep existing overview/delta/provenance behavior and contracts.
- Extend the external evaluation boundary to retain supported command
  completion/output evidence; unsupported fields remain `not_observed`.
- Define a separate task benchmark and answer/result contracts. Do not feed
  task/basic evidence into the existing direct/basic calibration importer.
- Add local real Git/SQLite/CLI fixtures, contract checks, distribution checks,
  and functional documentation of data flow and evidence limitations.

Out of scope: global skill rewriting, relaxing required skill reads, external
home-directory indexing, executing declaration-supplied commands, runtime
permission enforcement, real model execution, MCP, embeddings, source parsers,
LLM summarization, SQLite source-text storage, history pruning, and automatic
deployment or Git publication.

##### Completion Criteria

- An explicit task and target path produce only applicable context, including
  all applicable required items and their source identities.
- Selected source text matches the real file bytes/range and digest used to
  build the cue. Missing, changed, or ambiguous sources cannot produce a
  complete successful result.
- Required-item budget overflow fails explicitly. Optional omissions are
  counted and reported. Unknown matching inputs never silently exclude a
  potentially required item.
- Source references cannot escape the repository through absolute paths,
  traversal, or symlink targets.
- Delivery evidence, command success, correctness, and compliance have
  distinct fields and explicit availability. The implementation never equates
  a command name or final-response declaration with successful verification.
- Existing output/schema and real-path regressions pass without altering
  frozen calibration semantics or introducing a parser runtime dependency.
- T1-T8 and independent plan/implementation acceptance are recorded; Git
  landing evidence is required before marking the milestone Complete.

##### Dependencies And Decisions

D25-D30 define the accepted owner direction from the implementation and
measurement plan. G2 is Complete after the User confirmed L size / Mode B
on 2026-10-07. G4 owns acceptance and implementation authorization for the
reviewed named M4 plan and is Complete on 2026-10-08. M2 / G1 is a different
experiment and is not a behavioral dependency. M5 consumes the reviewed measurement
contracts. The User requested registration and trying the preceding plan on
2026-10-07; no real-model or Git-publication conditions were selected.

##### Implementation Plan

Plan Status: accepted
Plan Acceptance: 2026-10-08; User accepted plan20261008_052832, P001-P007, after both correction reviews.
Implementation Authorization: 2026-10-08; User authorized plan20261008_052832, P001-P007; auth20261008_072718 records the exact scope and exclusions.
Superseded Plan Artifacts: none

Named plan: plan20261008_052832, local review artifact
`work/review_sessions/20261008_050539_task-context/plan/plan20261008_052832_codex_gpt6_root.md`.
Review basis: HEAD `34d6d902365fb9ff4292996cafe0c18fd50c7d71` plus this
session's uncommitted milestone registration. Two initial independent reports
were read; convergence conv20261008_052115 includes all four measurement-plan
readiness findings. Both independent correction reviews accepted plan readiness;
owner acceptance and implementation authority are recorded under G4.

Accepted contract choices: OR within explicit task/path lists,
AND between dimensions, global dimensions and union across targets; canonical
component-aware relative paths; whole files or inclusive 1-based line ranges;
expected and snapshot full-file digests plus extracted-text digests; original
UTF-8/line endings retained; declaration/source symlinks, linked intermediate
components, gitlinks and untracked sources rejected. Every selected source/check
failure is an error. Required sources/checks/provenance remain whole under the
budget; whole optional units may be omitted with explicit IDs/count/reason.
These policies describe declared coverage, not all runtime instructions.

1. **P001 - Contracts and ADR.** Add six new schemas under `docs/schema/`:
   `task-context-v1.schema.json`, `task-cue-v1.schema.json`,
   `task-benchmark-v1.schema.json`, `task-answer-v1.schema.json`,
   `task-runner-v1.schema.json`, `task-evaluation-v1.schema.json`.
   Add `docs/TASK_CONTEXT.md`, `docs/task-context.example.json` and
   `docs/adr/0003-task-context-delivery-and-evidence.md`; extend schema tests.
   Keep existing versions unchanged. Verify T1 and T8.
2. **P002 - Selection and source.** Add
   `internal/taskcontext/{declaration,selection,source}.go` and tests.
   Bind tracked declaration/source/check bytes to expected and snapshot digests;
   use contained regular-file access and deterministic range selection.
   Source text stays transient; SQLite remains content-free. Verify T1-T3.
3. **P003 - Task cue and shipped CLI.** Add `internal/cue/task.go`,
   `internal/app/task_context.go`, tests and shipped fixtures under
   `internal/app/testdata/task-context/`; update app flags/help/dispatch.
   `cue --view task` requires `--context-file`, `--task-kind` and repeatable
   `--target`; legacy views reject task-only flags. Task mode rejects
   `--since`, `--path` and refresh-needed state. Build the complete mandatory
   envelope before optional budgeting. Test the actual executable over
   real Git/SQLite. Verify T2-T4 and T6.
4. **P004 - Benchmark and scorer.** Add `internal/taskbenchmark/`,
   `docs/benchmarks/task-context-v1.md` and three original task specifications.
   Bind prompt/tool/source/answer/assessment identity and required-check coverage.
   Explanation needs a source-backed rubric/assessment; instruction ID
   correctness and command success remain distinct. Missing policy or semantic
   evidence cannot award success or improvement. Verify T5.
5. **P005 - Task transport.** Add `internal/codexadapter/task.go` and tests;
   `cmd/repocue-codex-runner/main.go` dispatches only the exact task-runner
   contract to the new configuration. Legacy version/validators remain intact.
   Capture actual stdin, raw events/stdout/stderr and available completion/
   exit/output evidence on failure; decode JSON source text before comparing
   original bytes. Verify T5-T6.
6. **P006 - Attempts, costs and summaries.** Add
   `internal/evaluation/{task,task_attempt,task_cost}.go`, tests,
   `internal/app/task_evaluation.go` and executable pipeline tests.
   `evaluate-task` prepares without an agent by default; explicit runner mode
   performs one ordered pair, with no retry or continuation. Measure both
   actual cue CLI paths. Create external attempts before launch and preserve
   first/second-arm failure, malformed output, cancellation and post-failure
   basis errors. Retain wrong answers and unavailable data. Credit requires
   complete usage and compatible dated rates; a verdict also requires the
   frozen task-success/coverage policy and its evidence. Task reports cannot
   enter the old calibration importer. Verify T5-T6.
7. **P007 - Docs, checks and handoff.** Update `docs/CLI.md`,
   `DATA_MODEL.md`, `ARCHITECTURE.md`, `EVALUATION.md`,
   `CURRENT_STATUS.md`, `SUMMARY.md` and this detail. Run T1-T8 through
   actual packages/fixtures/binaries, obtain independent implementation
   acceptance, and prepare the G3 settings/evidence/policy checklist.
   No model execution or Git publication belongs to M4.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Contract | Real declaration/parser and JSON Schema checks for valid, invalid, duplicate, and unknown fields | Go | Versioned contracts agree with runtime validation |
| T2 | Selection | Shipped task cue CLI over temporary tracked Git repositories and real SQLite | Linux; Go | Global and task/path matches retain all required items; missing task inputs fail |
| T3 | Source integrity | Actual source files, selected ranges, source changes/deletions, symlinks, and refresh transitions through CLI | Real Git/SQLite/filesystem | Text and digest correspond; escaping, ambiguous, or stale sources fail |
| T4 | Budget | Shipped CLI with required overflow and optional omission fixtures | Real Git/SQLite | Required items never disappear into a successful cue; optional omission is explicit |
| T5 | Evidence | Built CLI and runner through actual task adapter/evaluator/scorer; outer executable captures stdin and actual-command outcomes plus failed/partial/missing event cases | Real Git/SQLite/subprocesses; only external Codex substituted | Source/check/prompt identity binds; failures survive; final-only/wrong-command/missing-policy/cheaper-wrong results cannot claim improvement |
| T6 | Regression | `make check` and `make test.race` | Linux; actual Go packages and fixtures | Existing and new behavior passes without internal-path substitutes |
| T7 | Distribution | `make build.linux-amd64` and `make build.windows-amd64`; inspect actual binaries | Linux amd64; CGo disabled | Static Linux distribution and Windows cross-build remain available |
| T8 | Presentation and acceptance | `make docs.build`, `git diff --check`, canonical row/detail/T-label checks, independent second/third-person verification | Source and generated book | Functional docs and plans match observed behavior and no unsupported savings claim appears |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | 2026-10-08 16:48:31 UTC | Linux; Go 1.27.1 | Passed | `make check` and `make test.race`: real embedded schema/parser fixtures, duplicate/unknown fields, independent task namespaces and unchanged legacy versions. |
| T2 | 2026-10-08 16:48:31 UTC | Linux; built CLI; real tracked Git/SQLite | Passed | Same full checks execute task/path/global/component/directory/new-file selection, union, unknown-input rejection and deterministic ordering through the shipped CLI. |
| T3 | 2026-10-08 16:48:31 UTC | Linux; real Git/SQLite/filesystem | Passed | Same full checks execute actual pins/ranges, CRLF/final newline/streaming UTF-8 boundaries, stale/deleted sources, refresh transitions, symlinks, gitlinks and invalid paths. |
| T4 | 2026-10-08 16:48:31 UTC | Linux; built CLI; real Git/SQLite | Passed | Same full checks execute exact fit, mandatory overflow with no successful stdout, whole optional omission, shared-source identity and complete provenance budgeting. |
| T5 | 2026-10-08 16:48:31 UTC | Linux; built CLI/runner/evaluator; external Codex fixture | Passed | Full checks execute actual delivery/checks and retained failures, cancellation, malformed/missing data, changed/unavailable basis, wrong answers, matched summaries and legacy rejection. Correction regressions bind actual provider schema bytes and reject changed or missing frozen pair schema, changed captured schema and changed request identity; before-second-arm mutation preserves the first arm and blocks the second provider. Absolute/relative/symlink summaries compare byte-identical. Only external Codex is substituted; synthetic counters/assessments establish handling, not capability or savings. |
| T6 | 2026-10-08 16:48:31 UTC | Linux; Go 1.27.1 | Passed | Correction `make check` and `make test.race` exit zero; formatting, package tests and vet pass. App executes 63.720s / 128.549s; evaluation 10.016s / 11.500s. Some unchanged packages use Go's cache; no claim of fresh execution of every package. |
| T7 | 2026-10-08 16:48:31 UTC | Linux amd64; CGo disabled; Windows amd64 cross-build | Passed | Correction `make build.repocue build.linux-amd64 build.windows-amd64` exits zero; actual `file`/`readelf` observes static Linux ELF with no INTERP and Windows PE32+. Windows execution is unverified. |
| T8 | 2026-10-08 17:10:29 UTC | Source and generated mdBook; independent Mode B lanes | Passed | Correction `make docs.build` and `git diff --check` exit zero; all nine M/G rows have exactly one detail, both M4 T-label sets are T1-T8, and all 15 local SUMMARY and 37 changed-document links resolve. Both second/third-person follow-ups accept hand20261008_165535: I1 fup20261008_170345 and I2 fup20261008_170912. Root reads both reports and rechecks all 52 target hashes at the recorded observation. Each lane executes current shipped fixtures and docs independently; no provider capability, billing or savings is inferred. |

Post-EOF fixture verification: the actual shipped CLI/runner and real Git/SQLite
fixtures passed `go test ./internal/app -run '^(TestTaskCueShipped|TestTaskEvaluationShipped)' -count=1 -v`
with exit 0, 12 top-level tests and package time 26.692s. The retained result
was rechecked at 2026-10-09 04:28:25 UTC; this is not a fresh full/race/build
rerun. Each of four source fixtures lost only one extra terminal LF; all other
accepted source bytes were preserved. Independent EOF checks accepted the
exact correction and current derivatives: E1 cmt20261009_041019 executed all
four shipped TaskCue tests, and E2 cmt20261009_041323 executed all eight shipped
TaskEvaluation tests. Only the external Codex provider is substituted.

##### Closure Evidence

- Completed on 2026-10-09 UTC: implementation, corrected T1-T8, both independent
  implementation acceptances, post-EOF fixture verification and Git landing
  satisfy M4 completion criteria. G2 and G4 are Complete; no linked issue exists.
- Source commit: `edd3dc23203f9b79c6cff9b9103a156716299549`, parent
  `34d6d902365fb9ff4292996cafe0c18fd50c7d71`. All 52 committed paths and blob
  hashes matched the accepted EOF inventory before publication.
- Observed at 2026-10-09 04:28:25 UTC after `git fetch origin`: local master,
  fetched `origin/master` and source commit above are identical, with ahead/behind
  counts 0/0 and a clean working tree before this record update. The actual
  upstream state and committed source paths were checked; a push exit alone
  is not used as landing evidence.
- Recheck with `git fetch origin`, `git rev-parse HEAD origin/master` and
  `git merge-base --is-ancestor edd3dc23203f9b79c6cff9b9103a156716299549 origin/master`.
  Later documentation commits may advance HEAD; the source commit must remain
  an ancestor of origin/master.
- No real paid-model run, actual provider-capability verification, account
  billing measurement or credit savings was observed. M5 remains Blocked by
  Open G3; M2 / G1 and their separate conditions are unchanged.

##### Plan Review Evidence

- Initial C1-C3 report rev20261008_051728: no confirmed current defect; contract
  choices separated from conditional implementation hazards.
- Initial C4-C6 report rev20261008_051835: four readiness gaps addressed by
  explicit dispatch, retained attempts, evidence/scoring and success/coverage
  prerequisites. Its actual legacy usage-presence pipeline and failure-
  publication checks passed; these do not establish M4 behavior.
- Named correction plan20261008_052832 has P001-P007 and exact V001-V010 commands mapped
  to T1-T8. Both independent correction checks and G4 are complete.
- C1-C3 correction report fup20261008_054213, finalized
  2026-10-08 05:42:13 UTC: no remaining confirmed plan defect; matching,
  source/range/digest/link/budget choices and preservation checks are reviewable.
- C4-C6 correction report fup20261008_054053, finalized
  2026-10-08 05:40:53 UTC: all four original readiness findings are accepted
  and reflected in the plan. No remaining verified plan blocker. Both reports
  were read by the coordinator; neither establishes product behavior or grants
  owner acceptance/implementation authority. Files remain local under
  `work/review_sessions/20261008_050539_task-context/reviews/`.
- Documentation preflight on 2026-10-08 05:34:26 UTC: `make docs.build` and
  `git diff --check` exited zero; the non-ASCII scan of this document, the named
  plan and convergence report found no matches. The generated book contains
  the M4 plan and G4 detail. These are plan/document observations, not completed
  M4 implementation checks; T1-T8 were Pending at that observation.

##### Implementation Review Evidence

- Initial I1 rev20261008_155804 accepts its source/selection/budget/preservation
  lane after independent second/third-person execution.
- Initial I2 rev20261008_155016 confirms F-codex_gpt6_r4-001: changing the
  archived answer schema leaves the original summary unchanged. Root executes
  the same retained real fixture and confirms the defect, plus relative-directory
  manifest rejection. Existing P001/P005/P006/P007 authority covers both
  implementation corrections; cmt20261008_160553 records the boundary.
- Correction regressions execute the shipped CLI/runner/outer-fixture/summary
  path. Actual frozen and provider-supplied schema bytes agree; missing/changed
  schema and changed request identity retain both attempts and suppress complete
  comparisons. A changed schema between arms preserves the completed first arm
  and prevents the second provider launch. Absolute, relative and symlink paths
  produce byte-identical summaries without launching providers.
- I1 follow-up fup20261008_170345, finalized 2026-10-08 17:03:45 UTC,
  accepts hand20261008_165535 with no remaining confirmed I1 defect. Fresh
  source/cue/public-example/frozen-schema tests, a separate real Git/SQLite
  source fixture, legacy contracts and actual static/cross-build inspections pass.
- I2 follow-up fup20261008_170912, finalized 2026-10-08 17:09:12 UTC,
  accepts the same target with no remaining confirmed I2 defect.
  F-codex_gpt6_r4-001 is verified implemented. The current Task suite and an
  independently compiled CLI/runner/shipped outer fixture execute five schema
  corruptions and physical-directory aliases. Raw argv/transport and artifact
  hash rebinding cannot override the frozen schema contract; two attempts stay
  visible and incomplete comparisons have no verdict.
- Root reads both actual reports and verifies all 52 hashes at
  2026-10-08 17:10:29 UTC. Reports remain local under
  `work/review_sessions/20261008_050539_task-context/reviews/`. Each report
  distinguishes fresh lane checks from earlier full/race checks, synthetic
  accounting and unobserved real-model/billing outcomes.
- The approved plan and authorization are unchanged. This completes the second
  implementation acceptance review and the sole correction-and-rereview cycle.
  Only canonical observations/acceptance/next-entry-point and the session index
  advance after acceptance; product code, schemas and functional docs stay fixed.

#### M5 - Task-Context Agent Comparison

Origin: 10a4f02 / M5
Identity History: none
GitHub Issue: none
Status: Blocked

##### Summary

Compare the frozen existing overview against the new task-specific cue on the
same repository state, model, tools, task, and execution settings. Measure
total credit estimate, tokens, full cue and agent duration, additional
discovery commands, required-context delivery, observable execution, and task
correctness. Evaluate quality independently of efficiency.

##### Scope

- Three task types: code explanation with original-source evidence,
  applicable instruction/reference identification, and actual execution and
  reporting of a specified verification command.
- One initial two-execution pair validates collection and cost accounting.
  Subject to its explicit continuation checkpoint, three repeats per task
  and two conditions total 18 executions including that first pair.
- Freeze alternating order before the pilot (2:1 within each task, 5:4 overall
  across nine pairs) and retain model/CLI versions, reasoning, speed
  tier, tool configuration, prompts, source digests, budgets, cache counters,
  repository fingerprints, attempt IDs, outputs, and scoring evidence.
- Separate source delivery, successful command execution, result
  correctness, and semantic compliance. Raw evidence remains external.

Out of scope: changing legacy M2 / G1, calibrating task results as direct/basic,
claiming long-conversation savings from fresh sessions, inferring cache misses,
model substitutions, automatic retries, and real agent source modifications.

##### Completion Criteria

- Every attempted run and failed/incomplete pair remains visible; no favorable
  or correct-only sample selection is permitted.
- Per-task paired cost differences, medians, ranges, correctness, missing
  measurements, and order/cache observations are reported separately.
- Freeze a digest-bound task-success and required pair/evidence coverage policy
  before the first model run. A verdict requires successful required task
  outcomes, no missing required context or coverage, preserved paired
  correctness and a lower paired-median credit estimate. Equal incorrect
  answers do not satisfy success. Missing semantic assessment stays unavailable;
  partial comparisons cannot award the complete planned improvement verdict.
  Three repeats support descriptive findings, not significance.
- Tokens include supplied context once. Cached tokens are an input subset;
  reasoning tokens are an output subset. Credit calculation uses uncached
  input, cached input, output, and the dated applicable credit rate/speed
  tier. Missing cost inputs suppress the cost estimate.
- Local CPU, memory, filesystem I/O, task-ready latency, and account billing
  remain unavailable unless their actual measurement path is added and run.
- Required evidence, first-pair continuation decision, and final acceptance
  are recorded before closure; no long-conversation claim is made.

##### Dependencies And Decisions

M4 supplies the reviewed implementation and measurement contracts. G3 owns
the exact real-model execution conditions and spending authorization; resume
as Not started. D25-D30 apply. The existing M2 / G1 experiment remains a
separate work identity with its original conditions and authorization.

Decision Date: 2026-10-09. The User requested Codex alongside Sonnet 5.5 and
selected Codex CLI. Use the existing overview/task comparison and credit
accounting. Do not add OpenAI API calls or import Claude cache-write accounting.
CLI selection does not select a model or authorize paid execution. Existing
runner usage and retained-input contracts remain unchanged.

##### Implementation Plan

Plan Status: draft
Plan Acceptance: none
Implementation Authorization: none
Superseded Plan Artifacts: none

1. Freeze the reviewed M4 artifact basis, task files, result schemas, applicable
   instructions, model/settings, output directory, and credit rate record.
2. Preflight both real cue paths and evidence capture without a model. Record
   full CLI output and timings, digests, source availability, and correctness
   checks; freeze the common cue budget before model results exist.
3. Execute only the first authorized A/B pair. Report usage, cost estimate,
   correctness, completeness, duration, and limitations. Wait for the explicit
   continuation decision before the remaining executions.
4. Run authorized remaining pairs with balanced order, original-source
   scoring, repository-state checks, and attempt/error retention.
5. Publish a separate versioned JSON result and human-readable comparison.
   Record observations and acceptance here. Do not import these conditions
   into the existing production direct/basic calibration.
6. After first-stage evidence, prepare a separately authorized continued-
   conversation experiment if required. A multi-turn runner and fixed turn
   sequence need their own plan; fresh-session results cannot substitute.

###### Proposed Codex CLI Companion

Plan Status: draft
Plan Acceptance: none
Implementation Authorization: none
Superseded Plan Artifacts: none

Named additive proposal: plan20261009_160442 under
`work/review_sessions/20261009_072038_prompt-caching/plan/`. P010 documents the
existing CLI measurement alongside Claude API evaluation in
`docs/PROMPT_CACHING.md` and `docs/EVALUATION.md`. P011 runs actual no-model
preparation for the first explanation A/B pair and records M5 / T1 evidence.
It preserves the original six-step M5 plan, balanced nine-pair schedule,
source pins, task-first prompt, runner and public schemas. Both items remain
unaccepted and unauthorized; no model run or successful preflight is claimed.

The measurements are total input, cached-input subset, output, optional
reasoning-output subset, complete dated credit estimate, cue/adapter duration,
supported commands, delivery and independent correctness. Cache counters
alone do not identify cached source text or establish causal cache savings.
CPU, memory, I/O, task-ready time and account billing remain unavailable.
Codex CLI executions can contain multiple provider requests. No TTL, explicit
cache boundary, output-token cap or hard billing ceiling is added. G3 still
requires exact model/settings, applicable account rates, assessment and
spending authority. Sonnet's proposed USD allowance does not apply.

Proposal review observed at 2026-10-09 16:21:55 UTC: C1 rsp20261009_161608 and
C2 rsp20261009_161706 each completed their first review of this companion
with zero findings and zero blockers. conv20261009_162155 records the result.
Plan acceptance/local authority remain pending; M5 T1-T4 are still Not run.
Actual proposal-document build and diff checks passed at 16:07:46 UTC.
No model, successful preflight, cache saving or account charge was observed.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Preflight | Actual existing and task cue CLI with frozen task/source/result contracts | Reviewed M4 basis; no model | Comparable inputs, complete required context, and reproducible digests |
| T2 | Pilot | First authorized pair through actual Codex and task evaluator | Pinned model/settings; external evidence directory | Usage, delivery, execution, and quality observations available or explicitly unavailable |
| T3 | Comparison | Authorized three tasks x three repeats x two conditions, including pilot | Identical paired repository state and balanced condition order | All attempts retained; paired cost and correctness findings re-derivable |
| T4 | Acceptance | Independently recompute results from actual logs and dated rates | Retained raw evidence | Missing data and subset accounting respected; no long-conversation or significance claim |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | Not run | Reviewed M4 basis | Pending | none |
| T2 | Not run | Actual Codex | Pending | G3 remains Open |
| T3 | Not run | Actual Codex | Pending | G3 and pilot continuation remain unresolved |
| T4 | Not run | Retained evidence | Pending | none |

##### Closure Evidence

- none; no real model execution or savings measurement has occurred.

#### G2 - Independent Review Session Configuration

Origin: 10a4f02 / G2
GitHub Issue: none
Status: Complete

##### Summary

Confirm the review-session size, execution topology, roles, and commit cadence
required before M4 changes durable schema and evidence contracts. The User
confirmed L size, Mode B, two independent-context Reviewers and the current
agent coordinating and implementing on 2026-10-07. The initial M-size/Mode-A
proposal was corrected before session creation. Publication is deferred to a
separately authorized single closure; no commit or push is delegated.

##### Completion Criteria

- User confirmation and required role/session entry are recorded.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| 2026-10-08 05:06:56 UTC | Confirmed | User selected L size / Mode B; session rs20261008_050539 records root Facilitator/Implementer and two independent Reviewer lanes |

##### Closure Evidence

- User configuration confirmation and role recording in local session `work/review_sessions/20261008_050539_task-context/README.md`; product plan acceptance and implementation authorization remain separate.

#### G3 - Task-Context Experiment Conditions And Cost Authorization

Origin: 10a4f02 / G3
GitHub Issue: none
Status: Open

##### Summary

Authorize the exact M5 experiment basis, model and reasoning/speed settings,
dated credit rates, evidence directory, first-pair execution count, spending
stop conditions, and continuation checkpoint. Authorization to develop or run
local tests is not authority for real paid model executions.

##### Completion Criteria

- Reviewed M4 evidence and exact M5 conditions are available.
- Freeze task/answer/check identities, source-backed explanation rubric and
  assessor provenance, success thresholds and required pair/evidence coverage
  before outcomes. Missing policy or semantic evidence cannot award a verdict.
- The maximum planned credit budget and enforceable stop controls are
  distinguished from estimates. If the adapter cannot enforce a hard billing
  ceiling, state that limitation instead of presenting a two-run checkpoint
  as a spending cap.
- User authorizes the first pair under these conditions. Further executions
  need the pilot continuation decision; failure does not authorize retries.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| Not run | Pending | No model, rate record, exact basis, or spending authority selected |

##### Closure Evidence

- none

#### G4 - Named Task-Context Plan Acceptance And Implementation Authorization

Origin: 10a4f02 / G4
GitHub Issue: none
Status: Complete

##### Summary

The User accepted and authorized reviewed M4 plan plan20261008_052832,
P001-P007, on 2026-10-08.
Responsible party: User. Affected work: M4. The prior request authorizes
registration and preparation; final schema/measurement choices remain explicit.
Both independent correction checks are complete. Owner acceptance and
implementation authorization are recorded separately in M4.

##### Completion Criteria

- The User accepts the current named plan, including its matching, source/
  range/digest/link policies and guarded evidence/comparison boundary.
- The User authorizes implementation of this exact P001-P007 plan; record
  acceptance and implementation authority separately in M4.
- Material changes require a superseding plan and new acceptance/authority.
  Paid M5/G3 and Git publication remain separate.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| 2026-10-08 07:27:18 UTC | Confirmed | User approved the named-plan question; auth20261008_072718 records P001-P007 and separate paid/Git exclusions |

##### Closure Evidence

- User acceptance and implementation authorization on 2026-10-08, recorded in
  `work/review_sessions/20261008_050539_task-context/plan/auth20261008_072718_codex_gpt6_root.md`; both independent correction reports were read before authorization.

#### M6 - Claude API Prompt-Caching Integration And Measurement

Origin: 10a4f02 / M6
Identity History: none
GitHub Issue: none
Status: Blocked

##### Summary

Add optional Claude API prompt-caching support around RepoCue context delivery.
The provider maintains the actual model-side cache. RepoCue prepares reusable
context, sends cache controls through an explicit adapter, and measures the
observed usage, request duration, estimated API cost and task quality.
The User accepted the reviewed local plan and separately authorized P001-P006
implementation on 2026-10-09. Paid execution remains separately gated by G6.

##### Scope

- Separate a deterministic reusable context prefix from changing task input,
  repository-state metadata and timestamps. Preserve required original sources,
  applicability, freshness and digest checks; a real source change must change
  the corresponding reusable content.
- Add an opt-in Claude API adapter that controls the actual submitted request
  and places `cache_control` at the intended reusable boundary. The provider
  requires a matching prefix and applies model/platform-specific minimum lengths
  and cache lifetimes [3]; cue output alone does not establish a cache hit.
- Retain exact submitted content, selected model/API settings, prefix identity,
  raw response usage, request timings, failures and missing measurements.
- Measure ordinary input, cache creation by applicable lifetime, cache reads
  and output separately. Define provider-specific accounting before mapping to
  shared reports; existing Codex input-subset semantics remain unchanged.
- Compare an uncached baseline, initial cache creation and subsequent reuse
  under matched task/context/settings. Include changed-prefix and expiry
  conditions, and classify each attempt from observed provider counters.

Out of scope: implementing the provider's model-side cache locally, controlling
existing Codex or Claude Code sessions, requiring a provider for ordinary CLI
use, replacing original context with LLM summaries, changing M2/G1 or M5/G3,
automatic paid retries, and claiming Codex-credit or account-billing savings
from Claude API estimates.

##### Completion Criteria

- The real adapter submits the intended reusable prefix and changing suffix;
  unchanged content is byte-stable and source changes preserve correctness.
- Raw usage retains the provider's distinct ordinary-input, cache-write,
  cache-read and output counters. Full-input totals count each category once;
  missing or incompatible counters suppress dependent estimates. Claude usage
  semantics are checked against the applicable official contract [3].
- Cost estimates use the selected model's dated rates, including cache-write
  lifetime differences, cache reads and outputs. Report initial creation cost,
  repeated-use totals and observed reuse count separately; no universal price
  multiplier or assumed hit is accepted.
- Authorized real-provider evidence establishes cache creation and reuse, plus
  changed-prefix and expiry behavior, from actual usage and timing observations.
  Retain every attempted request and task result. Local transport fixtures
  establish software handling only, not real cache behavior or savings.
- Paired comparisons retain source delivery and task correctness alongside
  efficiency. Missing quality evidence prevents an overall improvement claim;
  request duration is not labeled task-ready latency or account billing.
- Versioned compatibility, optional network use, Go implementation, CGo-free
  Linux static builds and Windows cross-builds are preserved.

##### Dependencies And Decisions

M4 supplies original-source task context and retained evidence boundaries.
G5 owns acceptance and implementation authority for the new integration plan;
G6 independently owns paid Claude API conditions and cost authority. G5 is
Complete; G6 is Open. Local implementation is authorized and locally verified; resume as
In progress after all external gates complete. D26-D30 apply.
Registration alone does not select a model, API version, TTL, schema, run count
or budget. The User selected Sonnet 5.5 on 2026-10-09; proposed local support
and paid conditions below still require their separate acceptance and authority.
The User selected M6 plan preparation as the current priority on
2026-10-09. Implementation and experiment order relative to M2 or M5 remains
unassigned.

##### Implementation Plan

Plan Status: accepted
Plan Acceptance: 2026-10-09; reviewed M6 local plan, serialized by plan20261009_075715
Implementation Authorization: 2026-10-09; explicit User implementation request, approval and continuation, recorded by auth20261009_075715; local P001-P006 only
Superseded Plan Artifacts: none

Accepted revision: 2026-10-09, plan20261009_075715. The User accepted this
reviewed architecture and authorized local P001-P006 implementation after
requesting implementation. auth20261009_075715 records that direction.
Independent detailed-contract cross-checks rsp20261009_080519 and
rsp20261009_080505 found zero confirmed blockers. The exact local field,
CLI and HTTP-limit details are in the named plan. G6 remains separate.

###### Premise And Boundary

| Class | Premise | Current Evidence | Planning Treatment |
| --- | --- | --- | --- |
| Confirmed constraint | Existing task transport and settings identify Codex, and task cost subtracts cached input from total input. | `internal/taskbenchmark/benchmark.go`: Settings, Usage and Transport; `internal/evaluation/task_cost.go`: TaskCredit | Add independent Claude contracts and USD estimates; never reinterpret the legacy fields. |
| Confirmed constraint | Current task prompts place the changing task before the complete cue, whose envelope contains snapshot and Git metadata. | `internal/evaluation/task_attempt.go`: taskPrompt; `internal/cue/task.go`: TaskEnvelope | Assemble a separate provider request from the validated cue, with stable context first and current provenance after the cache boundary. |
| Confirmed constraint | Existing retained-task verification binds the exact full cue and submitted prompt. | `internal/evaluation/task_integrity.go`: verifyTaskPair and verifyTaskAttempt | Preserve the old path and add separate reconstruction checks for the split Claude request. |
| Hypothesis | A stable prefix will reduce repeated-request cost for the selected model and task. | Provider rules [3]; no RepoCue real-provider observations | Test under G6. Do not treat prompt equality, local fixtures or a requested cache policy as a measured hit or saving. |
| Pending external authority | The User selected Sonnet 5.5; exact pilot conditions and costs are proposed, not authorized. | G6 remains Open; Sonnet 5.5 extension below | Freeze prepared identities and account scope after authorized local support; never infer paid authority from model selection. |

The accepted local slice is text context delivery and structured answers for
source identification or source-backed explanation. It does not build a
coding-agent runtime: repository writes, shell execution, client/server tools,
streaming, background warming and autonomous conversations are excluded.
Verification-command tasks remain unsupported in this slice and must not
receive fabricated completion evidence. G5 accepted these boundaries on
2026-10-09; the existing M5 task types and runner remain unchanged.

###### Contracts And Data Flow

Use Go standard-library HTTP behind an opt-in evaluation command. Keep the
core cue generator and SQLite content-free; do not add a provider SDK or
parser runtime dependency. Public contracts are independent of
task-runner-1, task-evaluation-1 and direct/basic calibration:

| Contract | Embedded File | Responsibility |
| --- | --- | --- |
| repocue/cache-benchmark-1 | `docs/schema/cache-benchmark-v1.schema.json` | Task selectors, quality policy, provider settings, cache conditions and dated USD-per-million-token rates |
| repocue/cache-request-1 | `docs/schema/cache-request-v1.schema.json` | Frozen context, wire request, expected answer schema, condition identity and digests |
| repocue/cache-observation-1 | `docs/schema/cache-observation-v1.schema.json` | Actual HTTP completion, raw response identity, nullable usage, timings and errors |
| repocue/cache-answer-1 | `docs/schema/cache-answer-v1.schema.json` | Task identity, required item IDs, claims with original-source IDs and uncertainty |
| repocue/cache-evaluation-1 | `docs/schema/cache-evaluation-v1.schema.json` | Preparation, attempt inventory, bound assessments, rates, derived metrics and summary |

The implemented local flow is:

```text
evaluate-cache preparation
  -> real refresh and cue --view task CLI
  -> freeze complete cue, repository evidence and answer schema
  -> deterministic prefix and current task/provenance suffix
  -> retain request body and digests without a network request
explicit run of one frozen step
  -> attempt-start journal and repository check
  -> Claude Messages API request
  -> retain raw response, timing, usage and failure state
  -> post-request repository check
summary
  -> validate retained artifacts and reconstruct delivery
  -> calculate usage, USD estimates and quality outcomes without a process or network request
```

The reusable prefix contains a fixed context-handling instruction and the
selected item/source records, including original text, full-file/range identity
and text digests. Sort deterministically; do not trim, summarize or silently
drop required text. Put repository-derived text in a user content block as
source material, preserving runtime instruction authority. Place an explicit
cache boundary after that block. Current task text, task/target selectors,
snapshot, branch/HEAD, freshness, observation time, budget and omission
accounting stay in the following suffix. Retain the complete original cue
separately; do not send the original source text twice. Unrelated Git metadata
changes may preserve prefix bytes, but must never bypass fresh source validation.

Freeze the system instruction, answer schema, model and request settings
across a comparable sequence. Hash the full reusable sequence, not just one
source block. A changed selected source or declaration pin changes that
identity; a local digest does not prove provider cache availability.
Account for the assembled request envelope when enforcing the context budget.
The existing byte-based estimate is not the model's cache-minimum tokenizer.

The direct Messages API supports explicit content-block cache controls and
non-streaming requests [4]. Use one complete response per explicit run,
with no tool or follow-up loop. Use a small provider-compatible answer schema
through `output_config.format` [5], retained and digest-bound before launch.
Validate the original frozen answer contract locally as well; provider schema
support never establishes source correctness or semantic compliance.
Unsupported selected-model/schema combinations stop preflight rather than
silently removing constraints or launching another request.

###### File-Level Steps

The P labels below are local to M6. P001-P006 are implemented in the local
worktree and verified below. P007 remains blocked by G6. Paths marked new
identify files introduced by this implementation; no landing is established.

| Plan Item | Files Or Subsystem | Intended Change | Verification |
| --- | --- | --- | --- |
| P001 | New `docs/adr/0004-provider-prompt-caching.md`; new `docs/schema/cache-*.schema.json`; schema contract tests | Promote the accepted provider/context/accounting decision; embed strict independent contracts and compatible public examples. Reject duplicate/unknown fields, trailing input and incompatible versions without changing existing schemas. | M6 / T1, T9 |
| P002 | New `internal/promptcache/context.go` and tests; existing `internal/cue` and `internal/taskcontext` APIs consumed unchanged | Decode a real task cue, assemble byte-stable prefix and current suffix, and validate required item/source coverage and total budget. Preserve exact original text and source freshness. | M6 / T2, T3 |
| P003 | New `internal/claudeadapter/client.go`, request/response handling and tests | Send the frozen non-streaming request with Go HTTP; load the API key only for explicit execution, keep credentials out of artifacts, reject redirects and retain bounded response/error bytes. Use context cancellation and one logical request, with no application retry. | M6 / T4 |
| P004 | New `internal/promptcache/usage.go`, `cost.go` and tests | Normalize disjoint Claude counters and lifetime-specific cache writes. Validate nullable counters, aggregate consistency, bounds and compatible dated decimal rates; calculate USD estimates without calling TaskCredit. | M6 / T5 |
| P005 | New `internal/evaluation/cache.go`, attempt/integrity/summary handling and tests | Freeze preparation, journal before launch, retain all failures and pre/post repository evidence, validate captured wire delivery and answer-schema identity, and summarize only retained data. Bind quality assessments to source, answer, task and attempt digests. | M6 / T6, T7 |
| P006 | New `internal/app/cache_evaluation.go` and shipped-path tests; command table in `internal/app/app.go`; `docs/CLI.md`, `docs/EVALUATION.md`, new `docs/PROMPT_CACHING.md`, `docs/SUMMARY.md` | Expose proposed evaluate-cache preparation, explicit run and read-only summary. Preparation needs no key or network; each run identifies one frozen step and enforces its request-count/output-token/time limits. Document unsupported measurements and the real API contract. | M6 / T1-T7, T9 |
| P007 | External G6 evidence; M6 Test Plan, Verification Results and Closure Evidence | After local checks and separate paid authority, collect the initial bounded requests and any separately continued baseline/change/expiry conditions. Report all attempts, matched costs and quality; obtain independent acceptance before M6 closure. | M6 / T8, T9 |

Detailed HTTP limits, exact schema fields, CLI flags and public examples are
recorded by accepted plan20261009_075715 and docs/PROMPT_CACHING.md. The locally
built binary exposes these commands. Existing schema/module/storage source is
unchanged, and regression checks passed. Local integration uses the actual
CLI, source extraction, SQLite and HTTP adapter with only an outer TLS server.

###### Usage And Cost Rules

Retain the provider fields without converting absence to zero:
`input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`,
`output_tokens`, and available `cache_creation` lifetime counters [3]. Check
that the lifetime sum agrees with aggregate cache creation when both exist.
For a request with a single frozen TTL, use that declared lifetime only when
the captured request and response establish a compatible interpretation;
otherwise lifetime-specific cost remains unavailable.

```text
total_input = ordinary_input + cache_write_5m + cache_write_1h + cache_read
estimated_usd = (ordinary_input * ordinary_rate
               + cache_write_5m * write_5m_rate
               + cache_write_1h * write_1h_rate
               + cache_read * read_rate
               + output * output_rate) / 1000000
```

When lifetime detail is unavailable, total input may still use the observed
aggregate cache-creation counter exactly once; cost needs a valid lifetime
allocation. Reject negative counters, overflow, incompatible units, model or
rate basis, and invalid rates. Cache creation is not counted again as ordinary
input, and aggregate writes are never added to their lifetime breakdown.
Reasoning details, if supported, remain an output subset and are not added twice.

Observed cache reads greater than zero establish reused tokens for that
request. Writes and reads may coexist. Zero observed cache activity does not
identify its cause; missing counters do not establish a miss. Preserve actual
response model, completion/stop state and supported request identity. A failed
or truncated answer can still incur usage; retain that cost separately from
task success. Missing complete usage suppresses full-attempt and sequence
cost totals instead of publishing a favorable subtotal.

Measure full request wall time and preparation/CLI duration separately;
non-streaming requests do not establish time to first token. Account billing,
CPU, memory, filesystem I/O and task-ready latency remain not_observed.
Compare complete matched cache-off request cycles against cache creation plus
all observed reuses, including initial write and output costs. Report per-pair
differences before descriptive aggregates and separate correctness. Changed
prefix and expiry probes are separate conditions, not extra favorable samples.
An observed saving applies only to the frozen conditions and reuse count.

###### G5 And G6 Execution Boundaries

G5 must accept the current named plan before local implementation begins.
G6 is separate authority for paid requests and is required for M6 completion;
it is not authority to modify code. M6 remains Blocked while either required
gate is Open. If separately authorized local work begins while G6 is Open,
record the actual partial progress and change the saved resume status to
In progress. Local P001-P006 progress and verification are recorded below.

Propose an initial two-request cache-creation/reuse pilot under exact G6
conditions. Observe its actual counters; the first request is not guaranteed
to be cold. That pilot alone cannot establish savings against a baseline.
Further baseline, changed-prefix and expiry requests require the agreed
continuation decision and a frozen schedule. Record request start times and
the latest observed cache access when judging TTL conditions; no automatic
sleep, warming, retry or continuation runs in the background.
The full request count, model/API versions, TTL/minimum length, workspace
scope, output limits, dated rates, assessment policy and spending controls
remain pending G6. A token or request limit is not a guaranteed billing cap.

###### Proposed Sonnet 5.5 Extension

Plan Status: draft
Plan Acceptance: none
Implementation Authorization: none
Superseded Plan Artifacts: none

Decision Date: 2026-10-09. The User selected `claude-sonnet-5-5` [6] during
execution-plan preparation. This additive proposal does not supersede the
accepted P001-P006 plan or its completed observations. That authorization does
not cover this profile extension. Named proposal: plan20261009_092451 under
`work/review_sessions/20261009_072038_prompt-caching/plan/`; its attachment
`sonnet55-pilot.json` is proposed configuration, not a prepared request.

The current supported-model list excludes Sonnet 5.5. Its default adaptive
thinking also differs from the current single-text-answer contract. Proposed
fixed settings are `thinking.type=between_tools` and
`output_config.effort=high`; without tools this model returns text [7].
Structured JSON remains the existing format [5]. Include these settings in
reusable identity and retain exact older profiles' wire bytes and identities.

| Plan Item | Files | Intended Change | Verification |
| --- | --- | --- | --- |
| P008 | `internal/promptcache/types.go`, `context.go` | Add only the exact Sonnet 5.5 profile and fixed thinking/effort fields; omit new fields for older models; unknown profiles still fail before HTTP. | Proposed extension of M6 / T1, T2, T4, T6 |
| P009 | `internal/app/cache_evaluation_test.go`, `docs/PROMPT_CACHING.md`, `docs/adr/0004-provider-prompt-caching.md` | Exercise real CLI prepare/run/summary with only outer TLS substituted; capture exact profile, preserve legacy bytes, retain failed usage/cost and document fixed settings. | Proposed extension of M6 / T1-T7, T9 |

Order: P008, P009 and local checks, real no-network preparation, exact G6 paid
approval, then the bounded pilot portion of P007. Exact local commands remain
the Planned Commands below; use `GOCACHE=/tmp/repocue-m6-go-build` in this
environment. Capture old wire/prefix identities from the real current path
before editing. New tests must fail on the missing profile/settings rather
than reconstructing the answer through an internal substitute. Current
recorded 4.x results do not establish Sonnet 5.5 support. No source padding,
adaptive thinking/tool runtime, public schema, module or storage change is
proposed. Material scope changes require a renewed plan and authority.

Proposal review observed at 2026-10-09 16:21:55 UTC: C1 rsp20261009_161608 and
C2 rsp20261009_161706 each completed their first review of P008-P009 and the
pilot conditions with zero findings and zero blockers. conv20261009_162155 records
that review only. P008-P009 acceptance/local authority and G6 paid authority
remain pending; no new profile verification or provider request occurred.

Preparation verification observed at 2026-10-09 09:29:12 UTC: the real existing
`build/bin/repocue evaluate-cache prepare` with the proposed benchmark exited 1
with `model has no supported frozen structured-output profile`. The API key
was removed from that process environment. DecodeSpec rejects before output
directory creation and HTTP; no prepared evidence directory exists. This is
the observed missing-support preflight, not a Sonnet 5.5 integration pass.
`make docs.build` exited 0 for the updated plan document. `git diff --check`
passed; the staged path list remained empty. The existing 25-file product
digest remained
`2fd82ca60fef4df964fc6b4abaf8e59eb19c721dc6ee6db04c53ee60f9d9abb6`.
The proposal attachment's JSON parameter checks passed; actual successful
preparation and Sonnet checks remain Pending. No broad source tests were
repeated for this documentation-only preparation.

###### Planned Commands

These commands identify verification targets; observed results are below.
Cache-specific orchestration checks reside in the actual shipped CLI tests
under internal/app, including T6 and T7, rather than an evaluation-package
TestCache filter with no matching tests. Existing evaluation regressions still
run through make check and make test.race.

| Check | Command |
| --- | --- |
| M6 / T1, T2, T3, T5 | `go test ./internal/promptcache -count=1` |
| M6 / T4 | `go test ./internal/claudeadapter -count=1` |
| M6 / T6, T7 | `go test ./internal/app -run '^TestCacheEvaluationShipped' -count=1` |
| M6 / T1-T7 | `go test ./internal/app -run '^TestCacheEvaluationShipped' -count=1` |
| M6 / T9 | `make check` |
| M6 / T9 | `make test.race` |
| M6 / T9 | `make build.repocue build.linux-amd64 build.windows-amd64` |
| M6 / T9 | `make docs.build` |
| M6 / T9 | `git diff --check` |

T8 commands are prepared only after exact G6 conditions exist. Build checks
must inspect the actual Linux ELF/no-INTERP and Windows PE artifacts; a Windows
cross-build does not establish native Windows execution.

##### Test Plan

| Label | Layer | Method | Environment | Expected Result |
| --- | --- | --- | --- | --- |
| T1 | Contracts | Embedded schema and strict parser tests plus actual CLI dispatch/preparation | New versioned fixtures and real built CLI; no key/network | Unknown/duplicate/trailing/incompatible inputs fail; no legacy contract changes or implicit paid request |
| T2 | Stable context | Shipped task-cue CLI through actual prefix/suffix assembly, repeating sources while changing task and Git metadata | Real Git/SQLite/filesystem | Same selected content gives stable prefix bytes; current metadata remains in suffix; original text appears once |
| T3 | Source integrity | Actual refresh/extraction and request preparation with changed pins, missing sources, symlinks, invalid ranges and mandatory budget overflow | Real Git/SQLite/source fixtures; HTTP boundary observed | No stale or incomplete successful request; source changes alter identity; zero provider launches on preflight failure |
| T4 | HTTP adapter | Built CLI through real adapter with only the outer HTTP transport replaced | Local TLS HTTP server; exact request capture, 4xx/5xx, partial body, cancellation and redirect fixtures | Correct frozen body/boundary/schema; one logical request without application retry; raw failures retained and credentials excluded |
| T5 | Accounting | Actual normalization and summary path for ordinary/write/read/mixed counters, lifetime sums, missing data, bounds and rates | Raw response fixtures carried through the adapter | Input/write categories counted once; compatible USD estimates or explicit unavailable results; unsuccessful task costs retained |
| T6 | Retained evidence | Shipped preparation/run/summary paths with tampered request, prefix, schema, response, assessment, directory aliases and changed repository state | Real files, CLI, Git/SQLite; only outer HTTP boundary substituted | Every attempt retained; incompatible evidence blocks comparison; summary launches no provider/process and does not mutate evidence |
| T7 | Quality | Actual submitted-content reconstruction and locally validated source-ID answers with bound independent assessments | Retained wire request/answer and original source references | Complete required delivery and separate correctness/compliance; malformed, wrong and unassessed answers stay visible; no command execution claim |
| T8 | Real provider | Authorized baseline, creation, reuse, changed-prefix and expiry requests with matched task results | Exact G6 Claude API/model/settings and budget | Cache activity supported by raw counters; complete matched costs/durations/quality or explicit unavailable findings |
| T9 | Regression | Real package/race checks, existing legacy contracts, Linux static/Windows cross-build inspection and docs build | Go and actual generated artifacts; no provider required | Existing behavior and distribution constraints preserved; Windows runtime and provider savings not inferred |

##### Verification Results

| Label | Observed At | Environment | Result | Evidence |
| --- | --- | --- | --- | --- |
| T1 | 2026-10-09 08:55:36 UTC | Go 1.27.1; embedded schemas and built CLI | Passed | Final make check and make test.race: five independent schemas, valid public examples, unknown versions/fields, duplicate/trailing JSON, unsupported model/API/count and incompatible dated rates; legacy schemas unchanged |
| T2 | 2026-10-09 08:55:36 UTC | Real Git/SQLite/task-cue/HTTP capture | Passed | TestCacheEvaluationShippedStablePrefixFreshnessAndBudget and FullCycleAndReadOnlySummary: stable prefix across changed prompt/Git basis; original sources once before boundary; source changes alter prefix |
| T3 | 2026-10-09 08:55:36 UTC | Real tracked source and preflight path | Passed | Missing/pin/range/symlink/mandatory and full-envelope budget failures; old prepared basis cannot launch HTTP; no source substitution |
| T4 | 2026-10-09 08:55:36 UTC | Built CLI and actual adapter; outer local TLS server | Passed | Frozen request/header/cache boundary/schema capture; 400/500, redirect, deadline, partial/oversized/malformed/trailing response, model mismatch and credential-echo retention rules; no retry; distinct absolute HTTP times survive attempted failures and remain null before HTTP |
| T5 | 2026-10-09 08:55:36 UTC | Raw TLS responses through actual summary; strict arithmetic unit cases | Passed | Ordinary/write/read and mixed 5m/1h counters; nullable/zero/bounds/lifetime checks; exact decimal USD, retained failed costs and unknown totals; initial/subsequent cost and observed reuse count separated |
| T6 | 2026-10-09 08:55:36 UTC | Shipped preparation/run/summary and actual files | Passed | Tampered benchmark/cue/schema/request/wire/response/assessment and HTTP interval/duration/null fields, aliases, pre/post basis changes and killed CLI journal remain invalid/visible; summary succeeds with repository unavailable and PATH/key empty, without changing evidence |
| T7 | 2026-10-09 08:55:36 UTC | Actual wire/answer/source and bound assessment fixtures | Passed | Required delivery distinct from correctness/compliance; wrong/unassessed/explanation answers and rejected assessment bindings remain visible; independent negative discovery assessment cannot be ignored; no observed reuse means no savings verdict |
| T8 | Not run | Actual Claude API | Pending | G6 remains Open |
| T9 | 2026-10-09 08:55:36 UTC | Linux Go 1.27.1; actual build/doc artifacts | Passed locally | Final make check and make test.race passed after timing correction; build.repocue, build.linux-amd64, build.windows-amd64 and docs.build exited 0 at 08:51:22 UTC; file/readelf inspection at 08:53:50 UTC showed static Linux ELF without INTERP and Windows amd64 PE32+; go mod tidy -diff and git diff --check passed; native Windows and provider behavior not tested |

##### Closure Evidence

- Local P001-P006 implementation and final T1-T7/T9 checks exist in the
  uncommitted worktree. Both independent implementation lanes accepted the
  corrected target with zero remaining blockers.
- Initial implementation reviews: C1 rev20261009_084615 accepted its lane;
  C2 rev20261009_084722 found F-codex_gpt6_cache_c2-001, dropped absolute HTTP
  timestamps needed for TTL evidence. The approved timing retention path is
  corrected and validated through the real CLI. Follow-ups fup20261009_085524
  (C1) and fup20261009_085910 (C2) independently verify that correction and
  accept the local implementation; each is review 2, the single correction
  cycle. Final full checks also passed after correction. Root rechecked the
  reviewed 25-file product digest at 2026-10-09 09:01:06 UTC:
  2fd82ca60fef4df964fc6b4abaf8e59eb19c721dc6ee6db04c53ee60f9d9abb6.
- G6 / T8 remains pending. No real-provider request, cache observation, account
  billing or real savings result exists. Synthetic TLS values verify software
  behavior only. M6 remains Blocked; no commit, push or landing evidence exists.
- ADR promotion: docs/adr/0004-provider-prompt-caching.md; operational contract:
  docs/PROMPT_CACHING.md. G5 acceptance does not authorize paid execution.

#### G5 - Prompt-Caching Plan Acceptance And Implementation Authorization

Origin: 10a4f02 / G5
GitHub Issue: none
Status: Complete

##### Summary

Responsible party: User. Affected work: M6. Accept the reviewed integration
plan, including the API boundary, reusable context rules, versioned evidence
and provider-specific accounting, then separately authorize implementation.
The request to register this milestone supplies neither acceptance nor
implementation authority. G6 remains the independent paid-execution gate.

##### Completion Criteria

- The current named plan is reviewed and accepted, with unresolved API,
  context, schema, credential and measurement decisions made explicit.
- Review the M6 draft revision dated 2026-10-09, including the proposed
  text-only API slice, independent contracts, P001-P007 and T1-T9. Independent
  design/measurement review precedes acceptance; this drafting action supplies
  no such acceptance.
- Implementation authorization is recorded separately from plan acceptance.
  Material changes require renewed acceptance and authorization.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| 2026-10-09 07:40:53 UTC | Pending | Independent draft reviews rev20261009_073105 and rev20261009_073440 completed with zero confirmed findings; conv20261009_073953 records convergence. Detailed contract choices, final named plan acceptance and implementation authorization remain unresolved |
| 2026-10-09 08:08:21 UTC | Passed | User accepted the reviewed M6 plan and separately authorized local implementation; plan20261009_075715 and auth20261009_075715 record the scope. Detailed-contract rsp20261009_080519 and rsp20261009_080505 independently found zero confirmed blockers. G6 paid authority remains excluded |

The reviewed M6/G5/G6 draft had SHA-256 `a0aa66695f01ada5541510c09f3dac53b56d6035832972e0c0d74b05d5d9b81b`.
The two final reports and convergence are under
`work/review_sessions/20261009_072038_prompt-caching/`. This observation changes
neither the M6 plan nor its draft/acceptance/authorization fields at that earlier
observation. At that time all M6 / T1-T9 remained Pending and no implementation,
paid request or saving was established. Current local observations are in M6.
At the earlier observation, exact schema fields/examples, CLI flags and HTTP
limits remained unresolved. The later accepted named plan serializes those
details within the reviewed architecture. Decision Date: 2026-10-09. The
original local P001-P006 implementation is authorized. The later Sonnet 5.5 selection
does not authorize the proposed P008-P009 profile extension or G6 paid calls.

##### Closure Evidence

- Decision Date: 2026-10-09. Accepted plan20261009_075715 and separately
  authorized local P001-P006 through auth20261009_075715. G6 remains excluded.

#### G6 - Claude API Experiment Conditions And Cost Authorization

Origin: 10a4f02 / G6
GitHub Issue: none
Status: Open

##### Summary

Responsible party: User. Affected work: M6 / T8. Authorize the exact paid
Claude API experiment independently of local implementation. Freeze provider
and model/API versions, repository/task/prefix identity, cache policy and TTL,
model minimum length, request order/count, dated API rates, evidence location,
task-quality assessment, spending controls and continuation conditions.

##### Completion Criteria

- Exact executable conditions distinguish baseline, write, reuse, changed
  prefix and expiry observations. No hit or miss is inferred from request
  order alone; the collection path retains provider usage and failed attempts.
- The User authorizes the initial bounded request set and its maximum planned
  cost. State any inability to enforce a hard billing ceiling; a run-count
  checkpoint is not a guaranteed spending cap. No automatic retry follows.
- Subsequent paid requests require the agreed continuation decision. G1 and
  G3 do not authorize this provider or experiment.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| Not run | Pending | Sonnet 5.5 selected on 2026-10-09; exact two-request conditions, dated rates and USD 0.10 allowance proposed below; no paid authority, prepared request or provider observation |

##### Proposed Initial Execution Conditions

Proposal Date: 2026-10-09. Acceptance: none. Paid Execution Authorization: none.
Named proposal: plan20261009_092451. Sonnet 5.5 local support P008-P009 must be
accepted, authorized and verified before actual preparation. The existing
binary rejects the selected model locally; no executable Sonnet request has
been prepared. G6 remains Open and M6 / T8 remains Pending.

| Parameter | Proposed condition |
| --- | --- |
| Provider / model / API | First-party Claude API / claude-sonnet-5-5 / 2023-06-01 |
| Endpoint | https://api.anthropic.com/v1/messages |
| Cache / profile | Explicit first-block ephemeral 5m; between_tools; high effort; existing structured JSON; no tools or streaming |
| Context | Real discovery cue; docs/task-context.example.json; target internal/cue; required repository-context item with original full README.md once |
| Schedule / limits | cache-create then cache-reuse; maximum two attempts; 512 output tokens, 60000 ms, 1048576 retained response bytes per request; full-wire local byte estimate limited to 8000 tokens |
| Evidence | Unique canonical non-linked child under /tmp/repocue-sonnet55-pilot; retain complete artifacts before temporary-storage cleanup |
| Prices | USD per million tokens, as of 2026-10-09: input 2.00, 5m write 2.50, 1h write 4.00, read 0.10, output 10.00 [8]; standard global first-party conditions |
| Allowance | Proposed USD 0.10; no enforced hard billing ceiling; paid approval pending |

Recheck README.md digest
`sha256:7ffa93ebe3f4cd35a600443026f79f8476b18e885bcb48bf5635e41faa15669e`
and declaration digest
`sha256:8f2d88708fc9afac9ce2a3a6b9a21881f9dc0e35028b703b6b83ec36fc924d59`
at real preparation. Freeze actual dirty repository basis, source/schema/
benchmark/wire/prefix identities and executable after all local changes.
Resolve and record the already initialized maintained cache root; no implicit
init, rebaseline, copy or repair. Confirm one workspace/key scope for both
requests without retaining the credential. Exact prepared identities and that
operator confirmation remain prerequisites for paid authorization.

Sonnet 5.5 has a 512-token cache minimum [6]. RepoCue's byte estimate does not
prove eligibility. Do not add filler or request token counting implicitly.
After the first response, summarize and stop on HTTP/answer/basis failure,
unknown cost, incomplete usage, model mismatch, timeout, no observed cache
activity or total input above the 8000-token planning scenario. Retain the
failed attempt. For conditional continuation, reprice the observed first total
input as full 5m write plus 512 output tokens; first observed cost plus that
second-call reserve must fit USD 0.10. This checkpoint is not a bill guarantee.

If separately authorized to continue, send the second request after the first
completes, preferably within 60 seconds, and before the first observed cache
write/read HTTP start plus five minutes. TTL starts at request start [3]. Stop
if the window cannot be met. No retry, automatic wait/warming, replacement
preparation or additional call is included. The first request may already read
cache; report actual counters, never infer cold creation or reuse from order.

Illustrative no-hit scenario: two requests, each with 8000 actual input tokens
all priced as 5m writes and 512 output tokens, cost
`2 * (8000 * 2.50 + 512 * 10.00) / 1000000 = USD 0.05024` [8]. This is a
conditional planning calculation, not observed usage, an invoice or a billing
bound. Actual provider tokenization/formatting overhead and account conditions
remain unverified.

Retain all attempts and separate source delivery, discovery ID correctness,
claim correctness and instruction compliance. Missing independent assessment
remains not_observed; no paid assessor is included. This pilot can establish
observed cache activity only. It has no matched baseline and leaves the full
baseline/change/expiry checks, savings, billing, task-ready latency and M6
closure unestablished. Further requests require a separate exact continuation
plan and authority.

##### Closure Evidence

- none

## Backlog

### Work

| Group | ID | Work unit | Type | Status | Ready | Deps | Done when / Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |

### Backlog Details

No unassigned work is recorded.

## References

[1] OpenAI. "Pricing." ChatGPT Learn. Accessed: Sep. 8, 2026. [Online]. Available: https://learn.chatgpt.com/docs/pricing

[2] OpenAI. "GPT-5.6 Luna Model." OpenAI API. Accessed: Sep. 8, 2026. [Online]. Available: https://developers.openai.com/api/docs/models/gpt-5.6-luna

[3] Anthropic. "Prompt caching." Claude Platform Docs. Accessed: Oct. 8, 2026. [Online]. Available: https://platform.claude.com/docs/en/build-with-claude/prompt-caching

[4] Anthropic. "Create a Message." Claude API Reference. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/api/messages/create

[5] Anthropic. "Structured outputs." Claude Platform Docs. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/build-with-claude/structured-outputs

[6] Anthropic. "Claude Sonnet 5.5." Claude Platform Docs. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/models/sonnet-5-5/overview

[7] Anthropic. "Claude Sonnet 5.5 migration guide." Claude Platform Docs. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/models/sonnet-5-5/migration-guide

[8] Anthropic. "Pricing." Claude Platform Docs. Accessed: Oct. 9, 2026. [Online]. Available: https://platform.claude.com/docs/en/about-claude/pricing
