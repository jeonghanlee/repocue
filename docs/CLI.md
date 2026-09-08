# RepoCue CLI Reference

## Scope

This document covers the current RepoCue CLI commands and their Makefile build wrappers.

**Out of scope:** MCP transport, production structural parsing, checkpoints, pruning, and filesystem watching.

The examples below assume that an installed `repocue` executable is available
in `PATH`. From a repository checkout, run `make build.repocue` and replace
`repocue` with `build/bin/repocue`. A direct Go build described in
[INSTALLATION.md](INSTALLATION.md) instead produces `./repocue`.

## Help

List every command with a one-line summary:

```bash
repocue help
```

Show the synopsis, summary, and flag defaults of one command:

```bash
repocue help cue
repocue cue --help
```

Help output is plain text on stdout with exit code 0. Running `repocue` without a command prints the same usage on stderr with exit code 2. Every other failure exits with code 1 and writes a single-line JSON error as the first line of stderr; an unknown command additionally prints plain-text usage guidance after that line.

---

## Repository State

Initialize the current Git repository with a deterministic full baseline:

```bash
repocue init .
```

Inspect the live Git basis and cached RepoCue state:

```bash
repocue status
```

Refresh changed tracked files and publish a new snapshot only when indexed state changed:

```bash
repocue refresh
```

Start a new epoch with a full baseline while retaining the superseded epoch:

```bash
repocue rebaseline --label milestone:first-prototype --reason manual
```

Except for the optional path accepted by `init`, state commands accept flags
only. Use `--repository PATH` where applicable and `--cache-dir PATH` to
override the normal cache location.

At an ordinary repository root, the CLI reuses a validated `location.json`
beside the SQLite database. A warm overview `cue`, `status`, or `refresh` needs
three Git calls; a missing or changed location hint requires discovery again.
Subdirectories, linked worktrees, and special Git configuration use the normal
discovery path. Location-cache write failures warn on stderr after the command
result without changing the JSON result or canonical snapshot state. If the
command fails, its JSON error remains the first stderr line, followed by the
warning.

Keep files and Git state unchanged while a command scans the repository.
RepoCue captures the scan's Git basis once, including when `status` or `cue`
checks freshness. If an editor or another process writes during the scan,
rerun `refresh` after it finishes before relying on the result.

---

## Cues

Generate a compact overview within an estimated token budget:

```bash
repocue cue --view overview --max-tokens 500
```

Generate a file-level repository-state delta from an earlier snapshot:

```bash
repocue cue --since snapshot-000001 --max-tokens 500
```

Experimental evaluation views remain available for measured comparisons:

```bash
repocue cue --view ranked --max-tokens 500
repocue cue --view provenance --path internal/ --max-tokens 500
```

Ranked facts are emitted only when the live repository still matches the
stored snapshot. A provenance cue reports `matched_files` and adds a
`provenance_files_omitted` warning when the token budget omits file records.

---

## Metrics and Evaluation

Read recorded baseline, refresh, and cue measurements:

```bash
repocue metrics
```

Run the model-neutral repository evaluation without external agent runners:

```bash
repocue evaluate --repository . --max-tokens 500
```

External runner and M2 experiment contracts are documented in
[EVALUATION.md](EVALUATION.md). The `repocue-codex-runner` binary is an
evaluation adapter and is not installed by the user installation target.

Run the five-condition M2 harness without an external agent runner:

```bash
repocue evaluate-m2 \
  --repository . \
  --oracle-tool tools/evaluation/structural-oracle.bash \
  --output-directory /tmp/repocue-m2-reports \
  --temporary-root /tmp/repocue-m2-work \
  --max-tokens 500
```

The output parent receives one generated report-set directory containing all
five condition reports. Output and temporary paths must be outside both the
evaluated worktree and its Git directory. RepoCue creates either parent when it
does not exist.

---

## Makefile Wrappers

Build and verify the CLI:

```bash
make check
make build.repocue
```

Preview and apply user installation:

```bash
make install.dry-run
make install.apply
make install.check
make uninstall.dry-run
make uninstall.apply
make uninstall.check
```

## Quantitative Reports

Read a report after initializing the repository; no agent is launched:

```bash
repocue report --repository .
repocue report --repository . --window 30d --format markdown
repocue report --repository . --consumers 7
repocue report --repository . --as-of 2026-09-01T00:00:00Z
```

Defaults are JSON, a 30-day UTC window, and 1, 2, 3, 5, and 10-consumer
scenarios. Positive `--consumers N` adds an explicit scenario. No profile means
maintenance only. One eligible profile is selected automatically; multiple
profiles require `--profile <calibration-id>` and otherwise suppress projections.
An unknown profile or missing/tampered selected artifact is an error.

`--as-of` accepts a non-future RFC3339 UTC instant. State and binding observations
must be strictly before it. Metric recording time governs activity and full
epoch maintenance; it is not database publication time. Current validation and
DB size remain separate current observations. Repeating historical results
requires the same DB/evidence basis; later retained observations can change
the result. Missing historical content evidence never comes from today's state.

An unchanged verified binding is `measured_reference`; a compatible later
snapshot is `estimated` with file/byte/language drift. Stale tracked state,
a new epoch, incompatible contract, or same-snapshot untracked-content change
makes the profile `not_applicable`. Report does not refresh. Ordinary refresh
cannot repair that untracked-content mismatch; renewed authorized evidence is
needed. Quality is dated source evidence, not a current assessment.

### Import External Calibration

Use a complete compatible direct/basic set from an authorized experiment.
Substitute your own external paths:

```bash
reports=/tmp/repocue-pairs
evidence=/tmp/repocue-pairs/calibration-evidence.json
repocue calibration import --repository . --reports "$reports" --evidence "$evidence"
```

Optional `--assessments /absolute/assessment-directory` validates the qualitative
files named in the evidence. Otherwise semantic quality is not assessed.
Import binds source content to the local current epoch/snapshot, not to a
coincidentally equal ID in the evaluator's temporary DB. Producer provenance
and exact digests are mandatory; historical values are never filled from the
current binary. The result contains calibration ID, relative path, and digest.
Reimport is idempotent.

Direct/basic preparation needs no structural oracle:

```bash
repocue evaluate-m2 --repository . --conditions direct,basic --output-directory /tmp/repocue-pairs
```

Without `--runner` this emits two dry-run reports, which cannot calibrate agent
savings. An authorized runner uses the existing `--runner` and `--task-file`
contract. Output and temporary workspace must remain outside the evaluated
worktree and Git directory. Import from the generated set directory and its
`calibration-evidence.json`, not the output parent. Multiple pairs require one
evidence document naming selected reports and contemporaneous records.
Import and report never implicitly run a model.

### Atomic Publication Wrapper

Report writes stdout and returns nonzero on failure. Do not redirect over the
last successful report. Save this optional Bash script as
`/home/example/bin/repocue-report.bash`, replace its four absolute paths, and
make it executable. It requires Bash, Git, `flock`, `mktemp`, `mkdir`, and `mv`.
It creates only its external output directory and retains failed drafts.
Keep the repository idle during execution. The example installs nothing.

```bash
#!/bin/bash
set -euo pipefail
export PATH=/usr/local/bin:/usr/bin:/bin
readonly repocue_binary=/home/example/.local/bin/repocue
readonly repository=/srv/project
readonly cache=/home/example/.cache/repocue
readonly report_dir=/home/example/reports/repocue
umask 077
mkdir -p -- "$report_dir"
exec 9>"$report_dir/report.lock"
flock -n 9 || exit 75
report_tmp=$(mktemp "$report_dir/.report.XXXXXXXX")
trap 'status=$?; if (( status != 0 )); then printf "Failed draft: %s\n" "$report_tmp" >&2; fi' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"$repocue_binary" report --repository "$repository" --cache-dir "$cache" --format markdown >"$report_tmp"
test -s "$report_tmp"
mv -f -- "$report_tmp" "$report_dir/current.md"
```

After verifying the wrapper manually, a user crontab entry can run it at 06:00:

```text
0 6 * * * /home/example/bin/repocue-report.bash
```

An alternative user systemd service and timer use the same lock and publisher:

```ini
# repocue-report.service
[Unit]
Description=Produce a RepoCue quantitative report
[Service]
Type=oneshot
WorkingDirectory=/srv/project
ExecStart=/home/example/bin/repocue-report.bash
```

```ini
# repocue-report.timer
[Unit]
Description=Run the RepoCue report daily
[Timer]
OnCalendar=daily
Persistent=true
Unit=repocue-report.service
[Install]
WantedBy=timers.target
```

Use cron or the timer, not both. Scheduling, permissions, retention, and
activation remain operator-owned. A held lock returns 75; command failure
leaves `current.md` unchanged. JSON and Markdown contain the same calculation,
classifications, and limitations. Markdown includes complete canonical JSON
as a data appendix.
