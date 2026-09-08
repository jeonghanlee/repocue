# RepoCue Work Register

## Scope

This document is the canonical work register for the M3 quantitative savings
report and its first repository calibration.

**Out of scope:** Implementation or external agent execution before their
separate authorization, and every feature listed in the M1 and M2 exclusions.

Release line: master
Milestone index: 10a4f02
Canonical path: `docs/milestone-10a4f02.md`
Canonical branch or ref: master
Git upstream: origin/master
Remote tracker: none

Next session entry point: prepare M1 closure acceptance and separately authorized Git landing; implementation and T1-T22 verification are independently accepted, while M2 and G1 remain unauthorized.

## Milestone

### Work

| Group | ID | Work unit | Type | Status | Ready | Deps | Done when / Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| M3 | M1 | Quantitative savings report contracts and implementation | Milestone | In progress | No | D1-D23 | The CLI imports a validated calibration, reports observed maintenance separately from projected agent savings, and passes M1 / T1-T22; [detail](#m1---quantitative-savings-report-contracts-and-implementation) |
| M3 | M2 | Initial repository calibration and report | Milestone | Blocked | No | M1, G1 | At least three controlled direct/basic pairs produce a reproducible calibration and first report without modifying the evaluated repository; [detail](#m2---initial-repository-calibration-and-report) |
| M3 | G1 | Calibration evidence authorization | External gate | Open | No | | The User authorizes either a verified retained report set or the cost and conditions of new Codex runs; [detail](#g1---calibration-evidence-authorization) |

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

### Milestone Details

#### M1 - Quantitative Savings Report Contracts And Implementation

Origin: 10a4f02 / M1
Identity History: none
GitHub Issue: none
Status: In progress

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
second-person documentation checks. M1 remains In progress pending formal
closure acceptance and separately authorized Git landing. No new real-agent
calibration or Git publication was performed.

##### Closure Evidence

- none

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

- M1 must be Complete.
- G1 must be Complete; resume as Not started after both dependencies complete.
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

##### Completion Criteria

- The evidence source, exact conditions, run count, repository basis, and any
  external execution cost are recorded with User authorization.

##### Verification Results

| Observed At | Result | Evidence |
| --- | --- | --- |
| Not run | Pending | User authorization and validated report-source record |

##### Closure Evidence

- none

## Backlog

### Work

| Group | ID | Work unit | Type | Status | Ready | Deps | Done when / Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |

### Backlog Details

No unassigned work is recorded.
