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
3. Run `repocue status --repository <path>`.
4. If the repository is not initialized, run `repocue init <path>`.
5. If status reports that a refresh is needed, run
   `repocue refresh --repository <path>`.
6. Generate the requested cue. Use the overview view and a 500 estimated-token
   budget when the user supplies neither:

   ```bash
   repocue cue --repository <path> --view overview --max-tokens 500
   ```

7. Read freshness, snapshot, epoch, branch, HEAD, and dirty-state provenance
   from the structured JSON before relying on its content.
8. Inspect repository files or Git state directly when the cue does not answer
   the request. Report important fallback inspection rather than implying that
   the cue contained information it did not provide.

## Request Mapping

Treat both direct CLI requests and natural-language requests as valid.

- "Show the current repository overview with RepoCue in about 500 tokens."
  maps to the overview command above.
- "Use RepoCue to understand this repository before reading files." maps to
  status, initialization or refresh when needed, and then an overview cue.
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
