---
name: repocue
description: Use the installed RepoCue CLI to inspect maintained Git repository context before direct discovery. Use when a user asks an agent to understand current repository state, produce a compact overview, check context freshness, or review changes since a snapshot.
---

# RepoCue Repository Context

Use RepoCue as the first source of repository-state context. Direct repository
inspection remains available when the cue is incomplete, uncertain, or stale.
The workflow requires the `repocue` executable and a Git repository.

## Workflow

1. Resolve the Git repository requested by the user. Use the current working
   directory only when no other path was supplied.
2. Verify that `repocue` is available in `PATH`. If it is unavailable, stop and
   direct the user to the
   [RepoCue Quick Start](https://github.com/jeonghanlee/repocue/blob/master/docs/QUICK_START.md)
   instead of installing it automatically.
3. Run `repocue refresh --repository <path>`. The command re-reads only
   changed tracked files and publishes a new snapshot only when the indexed
   state changed, which includes the Git basis (branch, HEAD, staged, unstaged,
   and untracked paths); otherwise it reports `"changed": false` and keeps
   the current snapshot.
4. If refresh fails because RepoCue is not initialized, run
   `repocue init <path>` and continue.
5. Generate the requested cue. Use the overview view and a 500 estimated-token
   budget when the user supplies neither:

   ```bash
   repocue cue --repository <path> --view overview --max-tokens 500
   ```

6. Read freshness, snapshot, epoch, branch, HEAD, and dirty-state provenance
   from the structured JSON before relying on its content.
7. Inspect repository files or Git state directly when the cue does not answer
   the request. Report important fallback inspection rather than implying that
   the cue contained information it did not provide.

## Request Mapping

Treat both direct CLI requests and natural-language requests as valid.

- "Show the current repository overview with RepoCue in about 500 tokens."
  maps to the overview command above.
- "Use RepoCue to understand this repository before reading files." maps to
  refresh, initialization when needed, and then an overview cue.
- "Is the RepoCue context fresh?" maps to
  `repocue status --repository <path>`. Read its `freshness` field, which
  is `current`, `dirty-but-indexed`, or `refresh-needed`. Status is
  read-only and never refreshes.
- A request for changes since a named snapshot maps to:

  ```bash
  repocue cue --repository <path> --since <snapshot-id> --max-tokens 500
  ```

`--max-tokens` controls the estimated cue-output budget. It is not the model's
complete context limit, and tokenizer-specific measured counts may differ.

## Boundaries

- Do not install, update, or remove RepoCue without an explicit user request.
- Do not run `rebaseline` unless the user explicitly requests a new epoch.
- Do not treat a stale cue as current.
- Do not prevent fallback repository inspection when correctness requires it.
