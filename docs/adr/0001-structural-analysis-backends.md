# ADR 0001: Structural Analysis Backend Boundary

Status: accepted for experiment; production backend not selected

Date: 2026-08-28

## Context

RepoCue requires a narrow structural-analysis boundary before any parser is
allowed into the production build. The default binary is currently CGo-free,
statically buildable for Linux, and cross-buildable for Windows amd64. M2 must
first determine whether structural facts reduce agent fallback work enough to
justify backend and indexing cost.

The initial boundary needs only language, file or module identity,
definitions, types, functions or methods, signatures, public symbols,
imports, and likely entry points. Full AST persistence, call graphs, data flow,
embeddings, and LLM summaries are outside this decision.

## Options Considered

### Official Tree-sitter Go Binding

`github.com/tree-sitter/go-tree-sitter` is the correctness and performance
reference. Its source imports C and embeds the Tree-sitter C runtime. Grammar
bindings also compile generated C sources. Making it mandatory would require
CGO and platform C toolchains, changing the current distribution contract.

Reference: <https://github.com/tree-sitter/go-tree-sitter>

### Pure-Go Tree-sitter-Compatible Runtime

`github.com/odvcencio/gotreesitter` describes a pure-Go runtime that loads
Tree-sitter parse-table data. It is a candidate for a later measured spike,
not an accepted dependency. Compatibility across required grammars, query
support, malformed-input behavior, update cadence, memory, and performance
must be compared with the official runtime.

Reference: <https://github.com/odvcencio/gotreesitter>

### WASM Through a Pure-Go Runtime

A Tree-sitter runtime and grammars can be compiled to WASM and hosted by
Wazero. Wazero is implemented in Go and does not require CGO, preserving
cross-build properties. The approach adds module packaging, runtime startup,
host-call, memory, provenance, and grammar-distribution costs. The pre-release
`github.com/malivvan/tree-sitter` project demonstrates this design but is not
selected as a dependency.

References:

- <https://github.com/wazero/wazero>
- <https://github.com/malivvan/tree-sitter>

### No Structural Backend

RepoCue can retain deterministic Git, filesystem, document ranking, entry-
point conventions, and recent-commit facts only. This preserves the smallest
runtime and deployment surface. It remains a valid production outcome if M2
does not show enough per-agent value to amortize parsing and indexing.

## Decision

No production structural backend is selected in M2. RepoCue keeps a narrow
conceptual boundary and uses an external Bash structural oracle only for the
experiment. The oracle is not linked into RepoCue, is not persisted in SQLite,
and does not change `go.mod`.

The official binding remains the reference implementation. Pure-Go and WASM
options remain candidates only after M2 demonstrates structural value.

## Consequences

- The default CGo-free Linux and Windows build properties remain unchanged.
- M2 can compare basic, ranked, and structural information under the same
  token estimate without committing to parser architecture.
- Oracle accuracy is intentionally limited and must be recorded as an
  experiment limitation.
- A later backend decision requires observed fallback, token, latency,
  baseline, refresh, memory, database-size, and distribution measurements.
- Any persistent structural index requires a separate storage-lifecycle
  decision. It must be keyed by stable entity and content-digest identity,
  store backend and version provenance, and update only changed source files.
