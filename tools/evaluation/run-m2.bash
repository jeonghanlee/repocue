#!/usr/bin/env bash

set -euo pipefail

function die {
    local message="$1"
    printf "run-m2: %s\n" "$message" >&2
    exit 1
}

function main {
    local repocue="repocue"
    local runner=""
    local task=""
    local oracle=""
    local reports=""
    local max_tokens="500"
    local run_index="1"
    local dry_run="0"
    local -a repositories=()
    local -a command=()
    local repository

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --repocue) [[ $# -ge 2 ]] || die "--repocue requires a value"; repocue="$2"; shift 2 ;;
            --repository) [[ $# -ge 2 ]] || die "--repository requires a value"; repositories+=("$2"); shift 2 ;;
            --runner) [[ $# -ge 2 ]] || die "--runner requires a value"; runner="$2"; shift 2 ;;
            --task) [[ $# -ge 2 ]] || die "--task requires a value"; task="$2"; shift 2 ;;
            --oracle) [[ $# -ge 2 ]] || die "--oracle requires a value"; oracle="$2"; shift 2 ;;
            --reports) [[ $# -ge 2 ]] || die "--reports requires a value"; reports="$2"; shift 2 ;;
            --max-tokens) [[ $# -ge 2 ]] || die "--max-tokens requires a value"; max_tokens="$2"; shift 2 ;;
            --run-index) [[ $# -ge 2 ]] || die "--run-index requires a value"; run_index="$2"; shift 2 ;;
            --dry-run) dry_run="1"; shift ;;
            --) shift; break ;;
            -*) die "unsupported option: $1" ;;
            *) repositories+=("$1"); shift ;;
        esac
    done

    [[ ${#repositories[@]} -gt 0 ]] || die "at least one repository is required"
    [[ -n "$oracle" ]] || die "--oracle is required"
    [[ "$max_tokens" =~ ^[0-9]+$ ]] || die "--max-tokens must be positive"
    ((10#$max_tokens > 0)) || die "--max-tokens must be positive"
    [[ "$run_index" =~ ^[0-9]+$ ]] || die "--run-index must be positive"
    ((10#$run_index > 0)) || die "--run-index must be positive"
    if [[ "$dry_run" != "1" ]]; then
        [[ -n "$runner" && -n "$task" ]] || die "--runner and --task are required unless --dry-run is used"
    fi

    for repository in "${repositories[@]}"; do
        command=("$repocue" "evaluate-m2" "--repository" "$repository" "--oracle-tool" "$oracle" "--max-tokens" "$max_tokens" "--run-index" "$run_index")
        if [[ -n "$reports" ]]; then
            command+=("--output-directory" "$reports")
        fi
        if [[ "$dry_run" != "1" ]]; then
            command+=("--runner" "$runner" "--task-file" "$task")
        fi
        "${command[@]}"
    done
}

main "$@"
