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

- Condition report: `repocue/evaluation-3`
- External runner observation: `repocue/evaluation-runner-3`
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

## Codex Adapter

`repocue-codex-runner` invokes a fresh ephemeral Codex non-interactive run in
JSONL mode with ignored user configuration and rules, read-only sandboxing,
the selected model and reasoning setting, and benchmark output schema.

Every `turn.completed.usage` event is preserved. Report totals sum the
observed per-turn values. Missing measurements remain absent and are not
converted to zero.

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

Projected totals are always `derived`. Estimated cue tokens remain the fast
runtime estimate `ceil(serialized UTF-8 bytes / 4)`. Measured tokenizer-
specific counts are separate fields.

M2 does not claim success before real condition reports and blinded
qualitative assessments exist. Dry runs verify contracts and execution paths;
they are not agent evidence.
