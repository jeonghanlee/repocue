# RepoCue — Project Background, Goals, Architecture, and Bootstrap Specification

## 1. Project Name

**RepoCue**

RepoCue is a local, agent-independent repository context cache for AI coding agents.

The name “Cue” reflects the primary goal: give an AI agent just enough current repository information to begin useful work immediately, without repeatedly re-reading and rediscovering the repository from scratch.

A concise description:

> **RepoCue maintains a compact, current, locally cached understanding of a Git repository and serves token-efficient context to multiple AI agents.**

A shorter tagline:

> **Read once. Refresh incrementally. Cue every agent.**

---

## 2. Background

Modern AI coding agents such as Codex CLI, Claude Code, Cursor agents, and other MCP-capable tools frequently begin work by rediscovering the repository.

A typical agent session may perform some variation of:

```text
git status
git log
find / tree
read README
read AGENTS.md
search architecture docs
inspect package manifests
grep symbols
read core source files
inspect tests
infer module relationships
inspect recent diffs
```

This is often necessary because the agent has no persistent, trustworthy view of the repository's current state.

The same repository may therefore be independently rediscovered:

- by multiple agents;
- by multiple sessions of the same agent;
- after relatively small code changes;
- after restarting an agent;
- after switching between tools;
- after a context window is compacted or discarded.

The result is repeated consumption of:

- LLM input tokens;
- agent reasoning tokens;
- file reads;
- grep/search operations;
- Git commands;
- tool calls;
- CPU time;
- filesystem I/O;
- latency before useful work begins.

For a repository that changes incrementally, repeatedly rebuilding the same high-level understanding is wasteful.

RepoCue is intended to remove as much of this repeated discovery cost as possible.

---

## 3. Core Problem

The problem is not simply “how to summarize a Git repository.”

Existing tools can already:

- pack repositories into a context file;
- generate code maps;
- index symbols;
- search source code;
- expose repositories through MCP;
- maintain embeddings;
- summarize documentation.

The specific problem RepoCue targets is:

> **How can the current state of a repository be maintained locally, refreshed cheaply, and communicated to many AI agents with the minimum practical token and latency cost while preserving sufficient accuracy and freshness for real work?**

RepoCue therefore focuses on **persistent current state**, **incremental refresh**, **multi-agent reuse**, **controlled rebaseline**, and **measurable resource savings**.

---

## 4. Primary Goal

RepoCue's primary goal is:

> **Automatically or manually maintain the current state of a Git repository locally and provide the freshest useful repository context to diverse AI agents with minimal tokens and minimal latency, thereby minimizing the repeated token, tool-call, compute, memory, and I/O costs associated with asking each agent to rediscover “the current state” of the repository.**

The optimization target is not merely smaller output.

RepoCue should optimize the total cost required for an agent to become **task-ready**.

---

## 5. Core Principle

The project can be summarized as:

```text
Read once
    ↓
Establish baseline
    ↓
Refresh incrementally
    ↓
Rebaseline intentionally
    ↓
Serve many agents
    ↓
Measure whether this is actually cheaper and still correct
```

Another useful formulation:

> **Index once, update incrementally, reset intentionally, serve many, and measure the savings without sacrificing context quality.**

---

## 6. What “Current Repository State” Means

“Current state” is broader than `git status`.

RepoCue should eventually understand several classes of repository state.

### 6.1 Git State

Examples:

- repository identity;
- current branch;
- HEAD commit;
- tags;
- dirty/clean working tree;
- staged changes;
- unstaged changes;
- untracked files;
- recent commits;
- changes since the current baseline;
- changes since a specified snapshot.

### 6.2 Code Structure

Examples:

- directory/module structure;
- languages;
- entry points;
- packages;
- important modules;
- public symbols;
- classes/functions/methods;
- signatures;
- imports;
- references;
- dependency relationships;
- test relationships;
- configuration relationships.

The initial implementation should favor deterministic structural analysis using Git, filesystem metadata, Tree-sitter, language tooling, or similar mechanisms rather than LLM analysis whenever possible.

### 6.3 Documentation State

Examples:

- README files;
- AGENTS.md or equivalent agent instructions;
- architecture documents;
- ADRs;
- design documents;
- installation documentation;
- release documentation;
- roadmap/milestone documents;
- API documentation;
- documentation-to-code relationships;
- potentially stale documentation.

RepoCue should distinguish between:

```text
document exists
document is important
document describes entity X
document changed
related code changed
document may now be stale
```

A document should not automatically be declared stale merely because related code changed. The system should preserve confidence and evidence.

### 6.4 Operational State

Where available:

- test status;
- build status;
- lint/static analysis state;
- generated artifacts;
- known TODO/FIXME markers;
- repository-local configuration;
- active development areas.

This category should remain extensible and should not block the MVP.

---

## 7. Key Design Decision: RepoCue Is a Cache, Not a Historical Archive

RepoCue does **not** need to permanently preserve every historical repository state.

Git already provides source history.

RepoCue's job is to maintain the most useful **current contextual understanding**.

Default retention should favor:

```text
current baseline
+
current state
+
short rolling delta history
+
explicitly pinned checkpoints
```

It should not keep unlimited snapshots by default.

---

## 8. Core Terminology

RepoCue should use the following terminology consistently.

### 8.1 Epoch

An **epoch** is a context-validity period with a common baseline.

Typical epoch boundaries include:

- a release change;
- a milestone;
- a major architecture transition;
- a user-requested reset;
- a tag selected as a new reference point.

Example:

```text
epoch: release:1.3.0
```

### 8.2 Baseline

A **baseline** is the result of a full repository analysis.

This is the “read once” state.

A baseline may include:

- repository structure;
- core symbols;
- dependency relationships;
- important documentation;
- derived summaries;
- repository facts;
- metadata required for incremental updates.

### 8.3 Rebaseline

A **rebaseline** discards the assumption that the previous baseline remains the best foundation and performs a new full analysis.

Rebaseline must be available manually.

Typical triggers:

```text
new release
new milestone
major refactor
large branch change
cache integrity concern
user request
significant context drift
```

Example CLI:

```bash
repocue rebaseline --reason release-change --label release:1.4.0
```

### 8.4 Snapshot

A **snapshot** identifies a coherent repository-context state at a point in time.

A snapshot does not necessarily duplicate all data. It may reference the current baseline plus accumulated deltas.

### 8.5 Delta

A **delta** records meaningful changes since another RepoCue state.

During the baseline and incremental-refresh stages, file-level operations such
as `file.content_changed` and `file.metadata_changed` are repository-state
semantic deltas. They describe the meaning of a repository state transition,
not code-symbol semantics. Symbol, dependency, and document semantic operations
may be added later without renaming the file-level operations.

The external delta should be semantic when possible.

Example:

```json
{
  "op": "symbol.signature_changed",
  "entity": "symbol:Runner.start",
  "before": "start(config)",
  "after": "start(config, wait=false)"
}
```

rather than merely exposing raw JSON paths.

### 8.6 Checkpoint

A **checkpoint** is a deliberately retained state.

Examples:

- release;
- milestone;
- Git tag;
- architecture transition;
- user-pinned state.

Checkpoint retention differs from normal rolling deltas.

### 8.7 Cue

A **cue** is the small agent-facing context projection RepoCue returns.

A cue is not the database and not the entire snapshot.

It is a token-budgeted view selected for a purpose.

Examples:

```text
overview cue
delta cue
structure cue
symbol cue
documentation cue
task-specific cue
```

---

## 9. Update Model

RepoCue must support both automatic and manual updates.

### 9.1 Manual Refresh

```bash
repocue refresh
```

This should:

1. inspect the repository state;
2. identify changed files;
3. update only affected structural/document/state records where practical;
4. create or advance the current snapshot;
5. record refresh metrics.

### 9.2 Automatic Refresh

Potential mechanisms:

- filesystem watcher;
- Git hooks;
- periodic local polling;
- explicit integration with agent/tool activity.

Potential Git hook points:

```text
post-commit
post-checkout
post-merge
post-rewrite
```

Automatic refresh must avoid excessive work during rapid edit sequences.

A debounce or batching mechanism is desirable.

### 9.3 Manual Full Read / Rebaseline

The user must always be able to say:

```bash
repocue rebaseline
```

or:

```bash
repocue rebaseline --label milestone:systemd-migration
```

This performs a new full analysis and establishes a fresh baseline.

### 9.4 Release Change

A release transition should normally create a new epoch.

Example:

```text
release 1.3 epoch
  baseline
  delta
  delta
  delta

release 1.4 begins
  optional pin of old release state
  new full baseline
  new epoch
```

RepoCue should support automatic detection heuristics but should not make destructive retention decisions solely from heuristics without clear policy.

---

## 10. Retention Model

Default:

```text
Current epoch
├── baseline
├── current snapshot
└── rolling recent deltas

Pinned checkpoints
├── release:1.3.0
├── milestone:systemd-migration
└── tag:v2.0.0
```

Suggested configurable policy:

```yaml
retention:
  rolling_deltas: 20
  keep_current_baseline: true
  keep_pinned_checkpoints: true
  prune_unpinned_previous_epochs: true
```

The exact policy should remain configurable.

---

## 11. Agent Independence

RepoCue must not be designed as a Codex-specific plugin.

It should be usable by:

- Codex CLI;
- Claude Code;
- Cursor;
- IDE agents;
- custom autonomous agents;
- future MCP-compatible clients;
- scripts and CI tools.

The same cached repository understanding should be reusable across all of them.

This is a fundamental project value.

---

## 12. Canonical Architecture

Recommended architecture:

```text
                        Git Repository
                              │
                ┌─────────────┴─────────────┐
                │                           │
            Git analyzer              Source/Doc indexers
                │                           │
                └─────────────┬─────────────┘
                              │
                        Context Engine
                              │
                 entities + relations + facts
                              │
                              ▼
                         Local Store
                           SQLite
                              │
                 ┌────────────┴────────────┐
                 │                         │
             Cue Projector             Evaluator
          scope/rank/budget          metrics/experiments
                 │
                 ▼
               MCP / CLI
                 │
        ┌────────┼─────────┐
        ▼        ▼         ▼
      Codex    Claude    Agent N
```

---

## 13. Canonical Data Model

The internal semantic model should be a lightweight typed property graph.

A graph database is not required for the first implementation.

SQLite tables can model entities and relations efficiently enough.

### 13.1 Entity Examples

```text
repository
directory
file
module
symbol
document
test
configuration
build_target
release
milestone
```

Example:

```json
{
  "id": "symbol:src/runner.py:Runner.start",
  "kind": "symbol",
  "subkind": "method",
  "name": "Runner.start",
  "path": "src/runner.py",
  "signature": "start(config: Config) -> Process",
  "content_digest": "sha256:...",
  "summary": "Starts an IOC process"
}
```

### 13.2 Relation Examples

```text
contains
defines
imports
calls
references
depends_on
documents
tests
configures
changed_by
affected_by
supersedes
```

Example:

```json
{
  "source": "doc:docs/architecture.md",
  "type": "documents",
  "target": "module:runner",
  "confidence": 0.93,
  "evidence": ["docs/architecture.md"]
}
```

---

## 14. Local Storage

Recommended MVP storage:

> **SQLite**

Potential supporting features:

- FTS5 for text search;
- JSON columns where useful;
- content hashes;
- indexes on entity IDs, paths, snapshot IDs, and relationships.

Suggested initial logical tables:

```text
repositories
epochs
baselines
snapshots

entities
relations
documents
facts

deltas
delta_items

checkpoints
checkpoint_items

refresh_runs
evaluation_runs
evaluation_metrics
```

Do not over-normalize the MVP prematurely.

Correctness, inspectability, and migration simplicity are more important than theoretical schema elegance.

---

## 15. Agent-Facing Format

The canonical agent interchange format should be:

> **Versioned structured JSON validated by JSON Schema**

Markdown should be generated as an optional human-readable view rather than used as the source of truth.

Example envelope:

```json
{
  "schema_version": "repocue/1.0",
  "kind": "overview",
  "repo": "example-repo",
  "epoch": "release:1.4.0",
  "snapshot": "s203",
  "basis": {
    "head": "71cf90a",
    "dirty": false
  },
  "freshness": {
    "status": "current"
  },
  "budget": {
    "max_tokens": 500,
    "estimated_tokens": 418
  },
  "summary": {
    "purpose": "Example repository",
    "architecture": [
      "cli -> config -> runner",
      "runner -> environment"
    ]
  },
  "warnings": [],
  "more": []
}
```

Important top-level concepts:

```text
schema_version
kind
repo
epoch
snapshot
basis
freshness
budget
content
warnings
references/more
```

---

## 16. Progressive Disclosure

RepoCue must not send all repository context merely because it is available.

The preferred information path is:

```text
manifest
   ↓
overview
   ↓
delta or focused structure
   ↓
specific entity/document
   ↓
original source only when necessary
```

Example approximate targets:

```text
manifest               50–150 tokens
overview              250–600 tokens
recent delta           50–500 tokens
focused structure     200–800 tokens
focused entity        100–500 tokens
```

These are initial hypotheses, not hard requirements.

They must be evaluated empirically.

---

## 17. Freshness and Trust

A compact context cache is dangerous if the agent cannot tell whether it is current.

Every meaningful cue should identify its basis.

Example:

```json
{
  "epoch": "release:1.4.0",
  "snapshot": "s203",
  "indexed_head": "71cf90a",
  "working_tree_state": "clean",
  "observed_at": "2026-08-27T20:00:00-07:00",
  "freshness": "current"
}
```

If RepoCue cannot guarantee freshness, it should say so.

Possible states:

```text
current
dirty-but-indexed
refresh-needed
possibly-stale
invalid
unknown
```

RepoCue should prefer explicit uncertainty over false confidence.

---

## 18. Documentation Freshness

Documentation state is important but must be handled carefully.

Potential status model:

```text
current
likely-current
possibly-stale
stale-confirmed
unknown
```

Deterministic signals may include:

- related code changed after the document;
- referenced files/symbols disappeared;
- documented signature differs from code;
- paths no longer exist;
- release version mismatch.

Semantic judgment may eventually use an LLM, but LLM usage should be optional and bounded.

The MVP should begin with deterministic evidence wherever possible.

---

## 19. LLM Usage Policy

RepoCue itself should minimize its dependence on LLMs.

Prefer deterministic extraction for:

```text
Git state
filesystem structure
symbols
signatures
imports
references
hashes
change detection
test metadata
document paths
explicit links
```

Optional LLM usage may later assist with:

```text
architecture summarization
document-to-code semantic linkage
change impact summaries
staleness judgment
task-specific context synthesis
```

If an LLM is used, RepoCue should measure and report that cost as part of its own maintenance cost.

Otherwise the system could appear to save agent tokens while silently spending similar resources during cache generation.

---

## 20. MCP Interface

RepoCue should expose its information through MCP for multi-agent interoperability.

The MCP interface should remain small.

Recommended initial tools:

```text
context_get
context_search
context_refresh
context_rebaseline
```

### 20.1 context_get

Possible views:

```text
manifest
overview
delta
structure
entity
document
task_context
```

Example request:

```json
{
  "view": "overview",
  "max_tokens": 500
}
```

Example delta request:

```json
{
  "view": "delta",
  "since_snapshot": "s198",
  "max_tokens": 400
}
```

### 20.2 context_search

Example:

```json
{
  "query": "IOC startup lifecycle",
  "scope": ["code", "docs"],
  "max_results": 8,
  "max_tokens": 700
}
```

### 20.3 context_refresh

Example:

```json
{
  "mode": "incremental"
}
```

### 20.4 context_rebaseline

Example:

```json
{
  "reason": "release-change",
  "label": "release:1.4.0",
  "pin_previous": true
}
```

MCP resources may later expose stable read-only URIs such as:

```text
repocue://<repo>/overview/current
repocue://<repo>/snapshot/<id>
repocue://<repo>/delta/<from>/<to>
repocue://<repo>/entity/<id>
repocue://<repo>/document/<id>
repocue://<repo>/checkpoint/<id>
```

---

## 21. CLI Interface

Initial CLI proposal:

```bash
repocue init
repocue status
repocue refresh
repocue watch

repocue cue
repocue cue --view overview
repocue cue --since s198
repocue cue --max-tokens 500

repocue search "startup lifecycle"

repocue rebaseline
repocue rebaseline --label release:1.4.0
repocue rebaseline --reason architecture-refactor

repocue checkpoint list
repocue checkpoint pin --label milestone:systemd-migration
repocue checkpoint delete <id>

repocue serve
repocue evaluate
repocue metrics
```

The exact CLI should evolve after the smallest vertical slice is validated.

---

## 22. Evaluation Is a Core Product Requirement

RepoCue must not claim success merely because its output is compact.

The system must quantitatively measure resource savings and qualitatively evaluate context quality.

The fundamental experiment is:

> Compare an agent that directly rediscovers the repository with an agent that uses RepoCue, using the same repository, model, and task.

---

## 23. Quantitative Metrics

At minimum evaluate:

### 23.1 Token Metrics

```text
direct discovery input tokens
RepoCue cue tokens
RepoCue maintenance LLM tokens, if any
total tokens per completed task
```

Important derived metrics:

```text
gross context token reduction
net total token reduction
```

### 23.2 Task-Ready Latency

Measure time from agent start to the point where the agent has enough repository understanding to begin the requested task.

This may require a practical operational definition.

### 23.3 Repository Reads

Count repository exploration operations attributable to current-state discovery.

Examples:

```text
file reads
grep/search calls
tree/find operations
git inspection calls
bytes read
files scanned
```

### 23.4 Tool Calls

Measure tool-call count before useful implementation begins and across the whole task.

### 23.5 Cache Maintenance Cost

Measure:

```text
refresh duration
CPU usage
memory usage
filesystem bytes read
filesystem bytes written
storage footprint
```

### 23.6 Freshness Lag

Measure:

```text
repository change time
→
RepoCue current-state availability time
```

### 23.7 Reuse

Measure:

```text
agents per baseline
sessions per baseline
cues served per refresh
```

### 23.8 Rebaseline Cost

Measure full reanalysis cost.

This is important for determining when incremental maintenance stops being cheaper than a new baseline.

### 23.9 Break-Even Point

Define:

```text
B = baseline creation cost
U = incremental maintenance cost
D = direct repository discovery cost per agent/session
C = RepoCue cue cost per agent/session
N = number of consumers
```

Compare:

```text
Direct total:
N * D

RepoCue total:
B + U + N * C
```

Estimate the reuse count at which RepoCue becomes cheaper.

---

## 24. Context Quality Metrics

Efficiency is meaningless if important context is lost.

Evaluate:

```text
accuracy
completeness
freshness
relevance
noise
stale-context error rate
agent correction rate
```

Possible qualitative dimensions:

- architecture understanding;
- current-state understanding;
- documentation awareness;
- recent-change awareness;
- task correctness;
- uncertainty handling;
- context usability;
- trustworthiness.

---

## 25. Failure Taxonomy

Track failures by category.

Initial taxonomy:

```text
missing-context
stale-context
incorrect-summary
over-compression
noisy-context
scope-confusion
invalid-delta
bad-document-link
false-freshness
rebaseline-too-late
rebaseline-too-often
```

A useful evaluation report should be able to say things such as:

> Token use fell by 84%, but public API impact was omitted in 6% of tested changes.

or:

> RepoCue had little benefit for a one-shot small repository, but became substantially cheaper after the second independent agent session.

or:

> Incremental refresh was effective for small commits, while a full rebaseline was cheaper after a large architecture migration.

---

## 26. Non-Goals

RepoCue is **not** initially intended to be:

- a replacement for Git;
- a complete source-code database;
- a permanent archive of every context state;
- a replacement for reading source code when implementation detail is required;
- a generic vector database;
- a hosted SaaS service;
- a Codex-only plugin;
- a Claude-only plugin;
- an autonomous coding agent;
- a repository packer that dumps the entire codebase into a prompt;
- an LLM-first repository summarizer.

RepoCue should help an agent decide **what it already needs to know and what it still needs to inspect**.

---

## 27. Security and Privacy

Default behavior should be local-first.

Do not transmit repository content to a remote service unless explicitly enabled.

Consider:

- `.gitignore`;
- secrets;
- generated files;
- binary files;
- private keys;
- credential files;
- vendor directories;
- build outputs.

The indexer should support exclusion rules.

No remote LLM dependency should be required for basic functionality.

---

## 28. Proposed Local Layout

Repository-local configuration:

```text
<repo>/
└── .repocue/
    └── config.yaml
```

Local state outside the repository:

```text
~/.cache/repocue/
└── <repo-id>/
    ├── state.db
    ├── objects/
    └── logs/
```

Reasons:

- cache data does not pollute source control;
- multiple agents share the same local state;
- repository config may optionally be committed;
- cache can be deleted and rebuilt;
- checkpoints can remain local unless explicit export is requested.

---

## 29. Suggested Configuration

Example:

```yaml
schema_version: 1

repository:
  include:
    - src/**
    - tests/**
    - docs/**
    - README*
    - AGENTS.md
  exclude:
    - .git/**
    - build/**
    - dist/**
    - node_modules/**
    - vendor/**

refresh:
  mode: manual
  watch_debounce_ms: 1500

retention:
  rolling_deltas: 20
  prune_unpinned_epochs: true

cue:
  default_max_tokens: 500

analysis:
  llm_enabled: false

evaluation:
  enabled: true
```

---

## 30. MVP Philosophy

Do not build the full vision before testing the core hypothesis.

The first implementation should prove:

> A locally maintained repository baseline plus incremental Git/file updates can materially reduce repeated agent repository discovery without materially harming task readiness.

The MVP should be narrow.

---

## 31. Proposed MVP

### Phase 0 — Repository Bootstrap

Create:

```text
README.md
LICENSE
pyproject.toml or equivalent
src/
tests/
docs/
```

Add architecture and schema documents before heavy implementation.

### Phase 1 — Git + Filesystem Baseline

Support:

```bash
repocue init
repocue status
repocue rebaseline
repocue refresh
repocue cue --view manifest
repocue cue --view overview
```

Baseline should capture:

- repository root;
- branch;
- HEAD;
- dirty state;
- tracked files;
- file sizes;
- file digests;
- directory structure;
- identified documentation;
- language/file-type distribution.

No LLM required.

### Phase 2 — Incremental Refresh

Detect changed files using Git and hashes.

Update only affected records.

Persist:

- baseline;
- current snapshot;
- rolling deltas.

Support:

```bash
repocue cue --since <snapshot>
```

### Phase 3 — Structural Code Index

Add Tree-sitter or an equivalent parsing layer.

Capture:

- symbols;
- signatures;
- imports;
- basic relationships.

Start with a limited set of languages if necessary.

Architecture must allow additional language adapters.

### Phase 4 — Documentation Relationships

Index key documentation.

Start with deterministic relationships:

- explicit file/path references;
- symbol names;
- headings;
- repository links.

Add conservative freshness warnings.

### Phase 5 — MCP Server

Expose current RepoCue data to external agents.

Start with:

```text
context_get
context_refresh
context_rebaseline
```

Add search only after basic retrieval is stable.

### Phase 6 — Evaluation Harness

Implement repeatable measurements comparing:

```text
direct repository discovery
vs
RepoCue-assisted discovery
```

The evaluation framework should be developed early enough that design decisions can be measured rather than guessed.

---

## 32. First Vertical Slice

The first meaningful end-to-end demonstration should be:

```text
1. Initialize RepoCue in a Git repository.
2. Perform one deterministic baseline scan.
3. Store it in SQLite.
4. Report repository and RepoCue status.
5. Produce a compact JSON overview cue within a requested token budget.
6. Detect changed files and run an incremental refresh.
7. Produce a semantic delta from a previous snapshot.
8. Rebaseline manually into a new epoch.
9. Measure duration, files scanned, bytes scanned, database size, cue bytes,
   and estimated cue tokens.
10. Maintain transactional consistency and explicit freshness and basis
    metadata.
```

Do not require MCP or Tree-sitter for this first slice.

The point is to establish the state model and measurement framework first.

The first slice retains a superseded epoch and its snapshots and deltas. It
does not implement checkpoint management, physical pruning, or retention-policy
enforcement. Those capabilities belong to the next storage-lifecycle
milestone, after the core state model is validated.

---

## 33. Important Architectural Constraints

### 33.1 Stable IDs

Entities need stable identifiers where practical.

Do not use database row IDs as public identities.

Example:

```text
file:src/runner.py
module:src.runner
symbol:src/runner.py:Runner.start
doc:docs/architecture.md
```

### 33.2 Content Hashing

Use content digests to avoid unnecessary reparsing.

### 33.3 Schema Versioning

All external structured outputs must carry a schema version.

### 33.4 Migration Support

SQLite schema changes should eventually use explicit migrations.

### 33.5 Deterministic Output

For a given repository state and configuration, deterministic portions of the cue should be stable.

Stable ordering helps:

- testing;
- diffability;
- prompt caching;
- reproducibility.

### 33.6 Token Budget

Token budgeting should be a first-class API concept.

Do not simply generate unlimited structured content and truncate arbitrary bytes.

Ranking and selection should happen before serialization.

### 33.7 Provenance

Derived facts should retain enough evidence to be inspected.

Example:

```json
{
  "fact": "docs/install.md may be stale",
  "confidence": 0.72,
  "evidence": [
    "src/install.py changed after document",
    "document references removed option --legacy"
  ]
}
```

---

## 34. Research Questions

The implementation should preserve the ability to answer these questions empirically:

1. How much repository discovery cost is repeated across agent sessions?
2. How much of that cost can RepoCue eliminate?
3. How small can a useful initial cue be?
4. When does aggressive compression begin harming task quality?
5. How frequently should incremental refresh occur?
6. When is rebaseline cheaper or safer than continuing incremental updates?
7. How many agent/session reuses are required to amortize baseline cost?
8. Does a shared context cache improve consistency between different agents?
9. How accurately can documentation freshness be estimated without an LLM?
10. What fraction of repository state can be maintained deterministically?
11. Which context categories produce the highest marginal value per token?
12. Does agent-driven follow-up retrieval outperform a larger initial cue?
13. What is the stale-context failure rate under realistic development activity?
14. How does RepoCue perform on small, medium, and very large repositories?
15. How much CPU/I/O overhead does maintaining the cache impose?

---

## 35. Acceptance Criteria for the First Vertical Slice

The first vertical slice is successful when it can demonstrate all of the
following on a real Git repository:

### Functional

- initialize repository metadata;
- perform a full baseline scan;
- persist the state locally;
- identify current Git HEAD and dirty state;
- report repository and RepoCue status;
- perform an incremental refresh;
- create a new snapshot;
- generate a compact overview;
- generate changes since a previous snapshot;
- manually rebaseline;
- start a new epoch;
- mark the previous epoch inactive or superseded without deleting it;
- commit each state transition transactionally.

### Performance / Measurement

The tool reports at least:

```text
baseline duration
refresh duration
files scanned
bytes scanned
database size
cue byte size
estimated cue tokens
```

### Quality

The output clearly exposes:

```text
which commit/state it represents
whether it is current
what is known
what is uncertain
what changed
where more detail can be obtained
```

Pinned checkpoints, pruning of unpinned historical state, checkpoint CLI
commands, and retention-policy enforcement are outside this acceptance scope.

---

## 36. Implementation Guidance for Codex

When implementing this project:

1. **Do not expand scope unnecessarily.**
2. **Do not begin with an LLM integration.**
3. **Do not begin with embeddings.**
4. **Do not begin with a graph database.**
5. **Use SQLite first.**
6. **Keep domain models independent of SQLite representations.**
7. **Make refresh/rebaseline behavior explicit and testable.**
8. **Treat measurement as part of the architecture, not an afterthought.**
9. **Prefer deterministic repository facts.**
10. **Keep agent-facing schema small, stable, and versioned.**
11. **Avoid creating large generated Markdown context files as canonical state.**
12. **Markdown may be generated as a view.**
13. **Keep MCP as an adapter over the core service, not the core itself.**
14. **Design so another transport can be added later.**
15. **Do not commit or push unless explicitly instructed by the user.**

---

## 37. Recommended Initial Technical Choices

These are recommendations, not irreversible requirements.

### Language

Go is the primary implementation language because:

- low startup latency and memory overhead;
- straightforward Git, filesystem, SQLite, and JSON integration;
- simple concurrency for later background refresh;
- simple cross-platform distribution as a single executable;
- suitable MCP support for a later adapter;
- direct measurement and profiling support.

Prefer the Go standard library where practical. SQLite should use
`database/sql` with a CGo-free driver so normal distribution does not require a
C compiler or a separate runtime. Keep the driver isolated inside the storage
package so measurements can justify a later change.

### Storage

```text
SQLite
```

### Serialization

```text
JSON
```

### Schema

```text
JSON Schema
```

### Configuration

```text
YAML or TOML
```

### CLI

Use a small mature CLI framework or standard library if sufficient.

### Testing

Use unit tests plus repository fixtures.

Integration tests should initialize temporary real Git repositories.

---

## 38. Suggested Repository Structure

One possible starting structure:

```text
repocue/
├── AGENTS.md
├── README.md
├── LICENSE
├── go.mod
│
├── docs/
│   ├── PROJECT_BRIEF.md
│   ├── ARCHITECTURE.md
│   ├── DATA_MODEL.md
│   ├── EVALUATION.md
│   └── ROADMAP.md
│
├── cmd/
│   └── repocue/
│       └── main.go
│
└── internal/
    ├── app/
    ├── repository/
    ├── storage/
    ├── snapshot/
    ├── cue/
    └── metrics/
```

Use package-level tests and temporary real Git repositories. Do not create
every package immediately merely because it appears in this tree.

Create modules as required by the vertical slice.

---

## 39. Suggested Initial AGENTS.md

Keep AGENTS.md short because every agent may repeatedly consume it.

Example:

```markdown
# RepoCue Agent Instructions

RepoCue is a local repository-context cache for AI coding agents.

Authoritative project specification:
`docs/PROJECT_BRIEF.md`

Core invariants:

- Maintain current repository context locally.
- Prefer incremental refresh over repeated full analysis.
- Support explicit manual rebaseline.
- Begin a new epoch for release/milestone resets when appropriate.
- Keep current state + short rolling deltas + pinned checkpoints; do not retain unlimited context history by default.
- Serve small, versioned, freshness-aware cues to multiple agents.
- Optimize total task-ready cost, not merely serialized output size.
- Measure token, latency, tool-call, CPU, memory, I/O, freshness, reuse, and context-quality outcomes.
- Prefer deterministic extraction; LLM use must be optional and measured.
- SQLite is the initial canonical local store.
- Structured JSON is the canonical agent interchange format.
- MCP is an adapter over the core, not the core architecture.
- Do not commit or push unless explicitly requested.
```

---

## 40. First Task for Codex

After reading this specification, Codex should **not immediately implement the entire system**.

The first task should be:

> Review this specification for internal contradictions, hidden assumptions, and decisions that would block the first vertical slice. Record only material issues. Then create the minimum repository skeleton and implement a deterministic Phase-1 prototype that can baseline a Git repository into SQLite, report status, create a compact JSON overview cue, perform a changed-file incremental refresh, and manually rebaseline into a new epoch while retaining the superseded epoch. Add metrics for duration, files scanned, bytes scanned, database size, output bytes, and estimated output tokens. Keep checkpoint management, pruning, retention-policy enforcement, MCP, Tree-sitter, embeddings, and LLM integration out of the first implementation unless a minimal abstraction is needed to avoid a clear architectural dead end.

The prototype should be runnable locally.

Suggested demonstration:

```bash
repocue init .
repocue status
repocue cue --view overview --max-tokens 500

# modify repository

repocue refresh
repocue cue --since <previous-snapshot>

repocue rebaseline --label milestone:first-prototype
repocue metrics
```

---

## 41. Questions Codex Should Resolve During Bootstrap

Codex may make reasonable implementation choices, but it should explicitly document decisions that materially affect the model.

Examples:

1. How is repository identity generated?
2. How are stable entity IDs represented?
3. What exactly makes two snapshots distinct?
4. How is working-tree state represented when HEAD has not changed?
5. What is the minimum data retained for a rolling delta?
6. How is token count estimated?
7. What does `current` freshness mean operationally?
8. How are ignored/generated/binary files handled?
9. How are schema migrations handled?
10. How should interrupted refreshes remain transactional?

Document answers in ADR-style short decisions where appropriate.

Safe pruning policy and pinned checkpoint representation are deferred to the
storage-lifecycle milestone and must not be enforced by the first slice.

---

## 42. Principle for Future Design Decisions

When deciding whether to add a feature, ask:

> **Does this feature measurably reduce the repeated cost of making AI agents understand the repository's current state, while preserving or improving correctness and freshness?**

If the answer is unclear, measure before expanding the architecture.

RepoCue's value is not the size of its index.

RepoCue's value is the **avoidable repository-understanding work that agents no longer have to repeat**.

---

## 43. Core Evaluation & Hardening Milestone

The accepted first vertical slice is followed by Core Evaluation & Hardening.
This milestone validates the central hypothesis before functionality expands:

> Does maintained repository context materially reduce the total resources
> required for an AI agent to understand the repository's current state?

### 43.1 State-Model Hardening

Integration coverage uses temporary real Git repositories and includes:

```text
modified tracked files
staged changes
untracked files
renames
deletions
ignored files
binary files
no-op refresh
branch changes
detached HEAD
dirty rebaseline
failed refresh
interrupted refresh
```

A failed or interrupted refresh must not expose a partially-current snapshot.
Filesystem and Git observations used by a published snapshot must be coherent
at the recorded observation point.

### 43.2 Evaluation Harness

The harness compares two arms:

```text
A. direct repository discovery by an agent
B. RepoCue-assisted repository discovery
```

The core harness is model-neutral. Optional external runners provide agent- or
tokenizer-specific observations through a versioned JSON contract. Missing
measurements are recorded as `not_observed`, never as zero.

Capture at least:

```text
RepoCue baseline and update cost
agent token consumption where observable
cue bytes and estimated tokens
tokenizer-specific measured token counts when supplied
repository files and bytes read
Git, search, and tool-call counts where observable
task-ready latency
fallback repository reads after a cue
context correctness and completeness findings
reuse cost for 1, 2, 3, 5, and 10 consumers
```

The runtime fast path retains `ceil(serialized UTF-8 bytes / 4)` as its token
estimate. Evaluation stores measured tokenizer counts separately and does not
couple core packages to an AI model or tokenizer.

Evaluation should use small, medium, and larger real Git repositories when
locally available. Repository probes are read-only. Optimization follows
repeatable measurement and identified bottlenecks rather than microbenchmarks.

MCP, Tree-sitter, embeddings, LLM integration, checkpointing, pruning,
retention-policy enforcement, and filesystem watching remain out of scope.

---

## 44. Real Agent A/B Validation Milestone

Core Evaluation & Hardening is accepted. The next milestone validates the
central hypothesis with real Codex CLI observations while retaining the
model-neutral external runner boundary.

The first adapter uses fresh ephemeral `codex exec` runs, JSONL event output,
an explicit read-only sandbox, and a stable structured response schema. Direct
and RepoCue-assisted arms use the same repository state, working directory,
model, reasoning effort, benchmark prompt, sandbox, and output schema. The
assisted arm receives the overview cue as initial context and may perform any
repository reads needed to resolve incomplete or uncertain context.

The first benchmark evaluates repository-state comprehension. It records
project purpose, branch, HEAD, dirty state, tracked and untracked changes,
entry points, components, documentation, recent relevant changes, and remaining
uncertainties.

Codex JSONL values are retained as observed measurements when present.
Classified commands, explicit file reads, combined totals, and reuse totals are
marked derived. Runtime cue token counts remain estimated. Missing values are
`not_observed`, never zero.

The deterministic scorer verifies branch, HEAD, dirty state, tracked changes,
and untracked paths against the coherent RepoCue basis. Semantic fields are
preserved for a separate qualitative assessment and are not presented as
deterministic truth.

Every pair records repository HEAD, RepoCue snapshot, Codex CLI version, model,
reasoning effort, benchmark version, output schema version, and agent command
observations. Repository state is checked before and after every agent run.

Reuse analysis projects 1, 2, 3, 5, and 10 consumers while separating baseline,
incremental refresh, cue, and fallback discovery costs. The central hypothesis
must not be declared validated until real observed agent measurements exist.

Tree-sitter, MCP, embeddings, RepoCue-internal LLM use, checkpointing, pruning,
retention-policy enforcement, and filesystem watching remain out of scope.
