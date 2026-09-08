# RepoCue Current Status

Updated: 2026-09-07

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

M3 local M1 is implemented, tested, and independently accepted in
[milestone-10a4f02.md](milestone-10a4f02.md). It remains In progress pending
formal closure acceptance and separately authorized Git landing. The implementation adds
calibration import, stdout-only JSON/Markdown reports, content-sensitive
evidence, and schema-3 references. Reports separate recorded maintenance from
paired future-consumer estimates and never launch an agent.

Real Git/SQLite/CLI fixture tests cover incompatible input, correctness,
optional assessments, untracked-content changes, publication rollback, history,
and cue process measurement. These use a synthetic runner, not observed
real-agent savings. T1-T22 passed; the canonical test matrix records the
executed checks and their limits. Initial real-agent calibration remains blocked
on G1; no new model runs are authorized.

## Deferred Scope

- MCP
- production Tree-sitter or other parser backend
- persistent structural index
- embeddings or LLM use inside RepoCue
- checkpoint management
- pruning and retention enforcement
- filesystem watching
