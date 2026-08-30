# RepoCue — Claude Code Instructions

RepoCue is a local, agent-independent repository context cache designed to reduce the repeated cost of AI agents rediscovering a repository's current state.

## Authoritative Documents

Read these in this order when project context is required:

1. `docs/CURRENT_STATUS.md` — current milestone, evidence, and immediate next work
2. `docs/PROJECT_BRIEF.md` — authoritative project goals and scope
3. `docs/ARCHITECTURE.md` — current architecture
4. `docs/DATA_MODEL.md` — epoch, baseline, snapshot, delta, and cue model
5. `docs/EVALUATION.md` — experimental methodology and measurements

`AGENTS.md` contains agent-independent development rules and also applies.

## Core Invariants

- RepoCue is implemented in Go.
- Keep the default distribution lightweight and preferably CGo-free.
- SQLite is the canonical local state store.
- Structured JSON is the canonical agent-facing representation.
- Prefer deterministic extraction over LLM-generated repository knowledge.
- Prefer incremental refresh over repeated full analysis.
- Manual rebaseline must always remain available.
- Release or milestone changes may begin a new epoch.
- Do not retain unlimited historical context by default.
- Optimize total agent task-ready resource cost, not just serialized cue size.
- Measurement is a core product requirement.
- MCP is an adapter over the core and is not yet the current implementation priority.

## Working Style

Do not assume the existing architecture or proposed next step is correct.

When reviewing design or experiments:

- distinguish observed, derived, estimated, and not-observed metrics;
- identify confounders;
- challenge conclusions when evidence is insufficient;
- prefer the smallest experiment that can falsify a design assumption.

Do not commit or push unless explicitly requested.