#!/usr/bin/env bash

set -euo pipefail

go_command="${GO_COMMAND:-go}"
module_file=""

function die {
    local message="$1"

    printf "error: %s\n" "${message}" >&2
    exit 1
}

function read_required_version {
    local key
    local value

    while read -r key value; do
        if [[ "${key}" == "go" && -n "${value:-}" ]]; then
            printf "%s\n" "${value}"
            return 0
        fi
    done < "${module_file}"
    return 1
}

function version_at_least {
    local current="$1"
    local required="$2"
    local current_major
    local current_minor
    local current_patch
    local required_major
    local required_minor
    local required_patch

    if [[ ! "${current}" =~ ^([0-9]+)\.([0-9]+)(\.([0-9]+))? ]]; then
        return 1
    fi
    current_major=$((10#${BASH_REMATCH[1]}))
    current_minor=$((10#${BASH_REMATCH[2]}))
    current_patch=$((10#${BASH_REMATCH[4]:-0}))

    if [[ ! "${required}" =~ ^([0-9]+)\.([0-9]+)(\.([0-9]+))? ]]; then
        return 1
    fi
    required_major=$((10#${BASH_REMATCH[1]}))
    required_minor=$((10#${BASH_REMATCH[2]}))
    required_patch=$((10#${BASH_REMATCH[4]:-0}))

    if ((current_major != required_major)); then
        ((current_major > required_major))
        return
    fi
    if ((current_minor != required_minor)); then
        ((current_minor > required_minor))
        return
    fi
    ((current_patch >= required_patch))
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --module-file)
            [[ $# -ge 2 ]] || die "--module-file requires a value"
            module_file="$2"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        -*)
            die "unknown option: $1"
            ;;
        *)
            die "unexpected argument: $1"
            ;;
    esac
done

[[ $# -eq 0 ]] || die "unexpected argument: $1"
[[ -n "${module_file}" ]] || die "--module-file is required"
[[ -r "${module_file}" ]] || die "module file is not readable: ${module_file}"

go_path="$(command -v "${go_command}" 2>/dev/null || true)"
if [[ -z "${go_path}" || ! -x "${go_path}" ]]; then
    printf "%s\n" "install Go from https://go.dev/dl/" >&2
    die "${go_command} is not an executable command"
fi

required_version="$(read_required_version)" || die "Go version is missing from ${module_file}"
current_version="$("${go_path}" env GOVERSION 2>/dev/null || true)"
current_version="${current_version#go}"

if ! version_at_least "${current_version}" "${required_version}"; then
    printf "Required Go version: %s\n" "${required_version}" >&2
    printf "Detected Go version: %s\n" "${current_version:-unknown}" >&2
    die "Go version does not satisfy the module requirement"
fi

printf "PASS: Go %s satisfies module requirement %s\n" "${current_version}" "${required_version}"
