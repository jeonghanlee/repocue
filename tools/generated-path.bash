#!/usr/bin/env bash

set -euo pipefail

action="${1:-}"
shift || true

kind=""
project_root=""
target=""
readonly realpath_command="${REALPATH_COMMAND:-realpath}"

function die {
    local message="$1"

    printf "error: %s\n" "${message}" >&2
    exit 1
}

function canonical_path {
    local path="$1"

    "${realpath_command}" -m -- "${path}"
}

function validate_target {
    local expected_docs_target
    local home_path

    [[ "${project_root}" == /* ]] || die "--project-root must be an absolute path"
    [[ "${target}" == /* ]] || die "--target must be an absolute path"
    [[ ! -L "${target}" ]] || die "generated target must not be a symbolic link: ${target}"

    project_root="$(canonical_path "${project_root}")"
    target="$(canonical_path "${target}")"
    home_path="$(canonical_path "${HOME}")"

    [[ "${target}" != "/" ]] || die "generated target must not be the filesystem root"
    [[ "${target}" != "${home_path}" ]] || die "generated target must not be HOME"
    if [[ "${project_root}" == "${target}" || "${project_root}" == "${target}/"* ]]; then
        die "generated target must not contain the project root: ${target}"
    fi

    case "${kind}" in
        build)
            if [[ "${target}" == "${project_root}/"* && "${target}" != "${project_root}/build" ]]; then
                die "build target inside the project must be ${project_root}/build"
            fi
            sentinel="${target}/.repocue-generated"
            ;;
        docs)
            expected_docs_target="${project_root}/docs/book"
            [[ "${target}" == "${expected_docs_target}" ]] || die "docs target must be ${expected_docs_target}"
            sentinel="${project_root}/docs/.repocue-book-generated"
            ;;
        *)
            die "unsupported generated path kind: ${kind}"
            ;;
    esac
    marker="repocue-generated:${kind}:${target}"
}

function validate_marker {
    local actual_marker=""

    [[ -f "${sentinel}" ]] || die "generated path is not marked: ${target}"
    IFS= read -r actual_marker < "${sentinel}" || true
    [[ "${actual_marker}" == "${marker}" ]] || die "generated path marker does not match: ${sentinel}"
}

function prepare_target {
    if [[ -e "${target}" ]]; then
        [[ -d "${target}" ]] || die "generated target is not a directory: ${target}"
        validate_marker
    fi

    if [[ "${kind}" == "build" ]]; then
        mkdir -p -- "${target}"
    else
        mkdir -p -- "${project_root}/docs"
    fi
    printf "%s\n" "${marker}" > "${sentinel}"
}

function clean_target {
    if [[ ! -e "${target}" && ! -e "${sentinel}" ]]; then
        printf "Generated path already absent: %s\n" "${target}"
        return 0
    fi

    validate_marker
    if [[ -e "${target}" ]]; then
        rm -rf -- "${target}"
    fi
    if [[ "${sentinel}" != "${target}/.repocue-generated" ]]; then
        rm -f -- "${sentinel}"
    fi
    printf "Removed generated path: %s\n" "${target}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --kind)
            [[ $# -ge 2 ]] || die "--kind requires a value"
            kind="$2"
            shift 2
            ;;
        --project-root)
            [[ $# -ge 2 ]] || die "--project-root requires a value"
            project_root="$2"
            shift 2
            ;;
        --target)
            [[ $# -ge 2 ]] || die "--target requires a value"
            target="$2"
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
[[ -n "${action}" ]] || die "action is required"
[[ -n "${kind}" ]] || die "--kind is required"
[[ -n "${project_root}" ]] || die "--project-root is required"
[[ -n "${target}" ]] || die "--target is required"

realpath_path="$(command -v "${realpath_command}" 2>/dev/null || true)"
[[ -n "${realpath_path}" && -x "${realpath_path}" ]] || die "${realpath_command} is required"

sentinel=""
marker=""
validate_target

case "${action}" in
    prepare)
        prepare_target
        ;;
    clean)
        clean_target
        ;;
    *)
        die "unsupported action: ${action}"
        ;;
esac
