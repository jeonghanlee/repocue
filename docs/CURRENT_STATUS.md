# RepoCue Current Status

Updated: 2026-10-08 UTC

## Accepted Foundation

RepoCue has an accepted Go vertical slice, Core Evaluation and Hardening, and
real Codex A/B validation. The maintained basic cue reduced observed tokens,
commands, explicitly named files, and elapsed time in the initial three-
repository experiment. The earlier derived byte proxy increased, and semantic
completeness remained unresolved.

The default distribution is CGo-free. Linux static and Windows amd64
cross-builds are required. SQLite remains the canonical local store.

## Scan Operating Assumption

Repository files and Git state must remain unchanged during an individual
scan. Each scan captures one Git basis; changes between scans are still
detected. SQLite transactional publication and checks before and after external
agent runs remain in place. See [Architecture](ARCHITECTURE.md#transaction-boundary).

A `status`, `refresh`, or overview `cue` at an ordinary repository root invokes
Git three times when its validated location cache is available. A cache miss
adds one discovery call. Subdirectory requests, linked worktrees, special Git
configuration, submodules, and newline-containing paths retain the required
additional queries. Core scan behavior and the persisted index digest format
are unchanged. Branch identity retains Git's short symbolic-ref format,
including disambiguated names when branches and tags share a name.

## Next Milestone

M3 local M1 and M3 are Complete in
[milestone-10a4f02.md](milestone-10a4f02.md), with independent implementation
and closure acceptance. Reporting landed in `a8f650c`; the token correction
landed in `8d61f6e`, which matches `origin/master` and GitHub master as observed
on 2026-09-08 UTC. The implementation adds calibration import,
stdout-only JSON/Markdown reports, content-sensitive evidence, and schema-3
references. Reports separate recorded maintenance from paired future-consumer
estimates and never launch an agent.

Real Git/SQLite/CLI fixture tests cover incompatible input, correctness,
optional assessments, untracked-content changes, publication rollback, history,
and cue process measurement. These use a synthetic runner, not observed
real-agent savings. T1-T22 passed; the canonical test matrix records the
executed checks and their limits. Initial real-agent calibration remains blocked
on G1; no new model runs are authorized.

The draft measurement plan compares three direct/basic pairs on RepoCue itself
using Codex 0.153.4, gpt-5.6-luna, medium reasoning, benchmark/answer version 2,
read-only execution, and a 500-token basic overview budget. The exact clean
commit and execution cost require approval. G1 records current rate references,
an explicitly assumed cost illustration, and a first-pair cost checkpoint;
actual token use, duration, and savings remain unmeasured.

The separately authorized local M3 correction preserves missing/null token
counters as `not_observed` and explicit zero as observed. New evaluations use
report/runner version 4; imports also validate unchanged version-3 reports
under their original contracts. Complete input/output values still produce a
total when cached or reasoning usage is absent.

The actual adapter/evaluator/import/report fixture path passed five presence
cases with matching JSON/Markdown measurements and preserved Git-fact scores.
A report set produced by the pre-change CLI was imported unchanged; producer
incompatibility still correctly prevents using it for projections. Full
tests, race, vet, tidy, static Linux and Windows builds, and mdBook passed;
independent correction and final second-person review were accepted with no
remaining finding. These verification results remain dated 2026-09-08 UTC;
the status update does not represent a new product test run.

No real model run is authorized. Writable local cache access remains a
measurement preflight requirement; the earlier read-only sandbox observation
has not been resolved. M2 remains Blocked on G1 approval; see the canonical
M2 preflight and G1 for the remaining measurement conditions and cost checkpoint.

## Task context implementation

Task context implementation provides separate version-1 contracts, pinned
original-source delivery, whole-unit budgeting, and an opt-in one-pair local
evaluation path. Built CLI/runner tests with real Git/SQLite and only external
Codex substituted cover successful checks, retained failures, cancellation,
source mutation, missing evidence, and cheaper wrong responses. Frozen and
provider-supplied answer-schema bytes are digest-bound. Summary rejects changed
schema/request evidence and resolves relative and symlink directory aliases.

M4 remains In progress until required distribution/documentation/independent
acceptance checks and Git landing are recorded in the canonical milestone.
M5 remains Blocked on M4 and G3. No paid task experiment, real provider evidence
capability, account billing, or task-context savings has been measured.
The [task benchmark reference](benchmarks/task-context-v1.md) lists the exact
paid preflight and proposed pilot/continuation schedule.

## Deferred Scope

- MCP
- production Tree-sitter or other parser backend
- persistent structural index
- embeddings or LLM use inside RepoCue
- checkpoint management
- pruning and retention enforcement
- filesystem watching
