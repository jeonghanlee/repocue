# RepoCue Agent Instructions

RepoCue is a local repository-context cache for AI coding agents.

Authoritative specification:
`docs/PROJECT_BRIEF.md`

Core rules:

- Prefer incremental refresh over repeated full repository analysis.
- Support explicit manual rebaseline.
- Release/milestone changes may start a new epoch.
- Retain current state, short rolling deltas, and pinned checkpoints only.
- Optimize total agent task-ready cost, not merely output size.
- Measure token, latency, tool-call, CPU, memory, I/O, freshness, and context quality.
- Prefer deterministic extraction; LLM use must be optional and measured.
- SQLite is the initial local store.
- Structured JSON is the canonical agent-facing format.
- MCP is an adapter over the core, not the core itself.
- Go is the RepoCue implementation language. Use Bash only for small,
  deterministic experiment tooling.
- Do not add Python project tooling or make a parser runtime a mandatory
  dependency without an accepted architecture decision and measurements.
- Preserve the default CGo-free Linux static build and Windows cross-build.
- Do not commit or push unless explicitly requested.
