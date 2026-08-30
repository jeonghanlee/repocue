# RepoCue Current Status

Updated: 2026-08-29

## Accepted Foundation

RepoCue has an accepted Go vertical slice, Core Evaluation and Hardening, and
real Codex A/B validation. The maintained basic cue reduced observed tokens,
commands, explicitly named files, and elapsed time in the initial three-
repository experiment. The earlier derived byte proxy increased, and semantic
completeness remained unresolved.

The default distribution is CGo-free. Linux static and Windows amd64
cross-builds are required. SQLite remains the canonical local store.

## Deferred Scope

- MCP
- production Tree-sitter or other parser backend
- persistent structural index
- embeddings or LLM use inside RepoCue
- checkpoint management
- pruning and retention enforcement
- filesystem watching
