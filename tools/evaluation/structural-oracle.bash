#!/usr/bin/env bash

set -euo pipefail

export LC_ALL=C

function die {
    local message="$1"
    printf "structural-oracle: %s\n" "$message" >&2
    exit 1
}

function cleanup {
    local exit_code=$?
    if [[ -n "${oracle_workspace:-}" && -d "$oracle_workspace" ]]; then
        rm -rf -- "$oracle_workspace"
    fi
    exit "$exit_code"
}

function source_rank {
    local path="$1"
    case "$path" in
        src/*.py) printf "%s" "10" ;;
        tests/*.py) printf "%s" "30" ;;
        *.py) printf "%s" "20" ;;
        *) printf "%s" "20" ;;
    esac
}

function extract_python {
    local root="$1"
    local path="$2"
    local output="$3"
    local rank="$4"
    local full_path="${root}/${path}"
    if ! grep -Iq -E '^[[:space:]]*(class|def|async[[:space:]]+def|from|import)[[:space:]]' "$full_path"; then
        return 0
    fi
    awk -v rank="$rank" -v module="$path" '
        function clean(value) {
            gsub(/\t/, " ", value)
            sub(/[[:space:]]+$/, "", value)
            return value
        }
        /^[[:space:]]*class[[:space:]]+[A-Za-z_][A-Za-z0-9_]*/ {
            line=$0
            sub(/^[[:space:]]*class[[:space:]]+/, "", line)
            name=line
            sub(/[(:].*$/, "", name)
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", rank, "Python", module, "class", name, clean($0)
            next
        }
        /^[[:space:]]*(async[[:space:]]+)?def[[:space:]]+[A-Za-z_][A-Za-z0-9_]*/ {
            line=$0
            indent=(line ~ /^[[:space:]]/)
            sub(/^[[:space:]]*(async[[:space:]]+)?def[[:space:]]+/, "", line)
            name=line
            sub(/\(.*/, "", name)
            kind=(indent ? "method" : "function")
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", rank, "Python", module, kind, name, clean($0)
            next
        }
        /^[[:space:]]*(from|import)[[:space:]]+/ {
            line=$0
            sub(/^[[:space:]]*/, "", line)
            name=line
            sub(/[[:space:]].*$/, "", name)
            if (name == "from" || name == "import") {
                rest=line
                sub(/^(from|import)[[:space:]]+/, "", rest)
                name=rest
                sub(/[[:space:],].*$/, "", name)
            }
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", rank, "Python", module, "import", name, clean($0)
        }
    ' "$full_path" >>"$output"
}

function extract_bash {
    local root="$1"
    local path="$2"
    local output="$3"
    local full_path="${root}/${path}"
    if ! grep -Iq -E '^[[:space:]]*(function[[:space:]]+|[A-Za-z_][A-Za-z0-9_]*[[:space:]]*\(\)|source[[:space:]]+|\.[[:space:]]+)' "$full_path"; then
        return 0
    fi
    awk -v module="$path" '
        function clean(value) {
            gsub(/\t/, " ", value)
            sub(/[[:space:]]+$/, "", value)
            return value
        }
        /^[[:space:]]*function[[:space:]]+[A-Za-z_][A-Za-z0-9_]*/ {
            line=$0
            sub(/^[[:space:]]*function[[:space:]]+/, "", line)
            name=line
            sub(/[[:space:]({].*$/, "", name)
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", "20", "Bash", module, "function", name, clean($0)
            next
        }
        /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*\(\)[[:space:]]*\{/ {
            line=$0
            sub(/^[[:space:]]*/, "", line)
            name=line
            sub(/[[:space:]]*\(\).*/, "", name)
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", "20", "Bash", module, "function", name, clean($0)
            next
        }
        /^[[:space:]]*(source[[:space:]]+|\.[[:space:]]+)/ {
            line=$0
            sub(/^[[:space:]]*(source[[:space:]]+|\.[[:space:]]+)/, "", line)
            name=line
            sub(/[[:space:];].*$/, "", name)
            gsub(/^["\047]|["\047]$/, "", name)
            printf "%s\t%s\t%s\t%s\t%s\t%s\n", "20", "Bash", module, "dependency", name, clean($0)
        }
    ' "$full_path" >>"$output"
}

function main {
    local supplied_root="${1:-}"
    local root
    local ranked_output
    local sorted_output
    local path
    local rank

    [[ -n "$supplied_root" ]] || die "usage: structural-oracle.bash REPOSITORY"
    [[ $# -eq 1 ]] || die "exactly one repository path is required"
    root=$(git -C "$supplied_root" rev-parse --show-toplevel) || die "not a Git repository: $supplied_root"
    root=$(realpath "$root")
    oracle_workspace=$(mktemp -d)
    ranked_output="${oracle_workspace}/ranked.tsv"
    sorted_output="${oracle_workspace}/sorted.tsv"
    : >"$ranked_output"

    while IFS= read -r -d '' path; do
        [[ "$path" != *$'\t'* && "$path" != *$'\n'* ]] || continue
        case "$path" in
            *.py)
                rank=$(source_rank "$path")
                extract_python "$root" "$path" "$ranked_output" "$rank"
                ;;
            *.sh|*.bash)
                extract_bash "$root" "$path" "$ranked_output"
                ;;
        esac
    done < <(git -C "$root" ls-files -z -- '*.py' '*.sh' '*.bash')

    sort -t $'\t' -k1,1n -k3,3 -k4,4 -k5,5 -k6,6 "$ranked_output" >"$sorted_output"
    while IFS= read -r line || [[ -n "${line:-}" ]]; do
        printf "%s\n" "${line#*$'\t'}"
    done <"$sorted_output"
}

oracle_workspace=""
trap cleanup EXIT
main "$@"
