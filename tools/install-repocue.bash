#!/usr/bin/env bash

set -euo pipefail

readonly ACTION="${1:-}"
shift || true

source_path=""
destination=""
command_name="repocue"
readonly install_command="${INSTALL_COMMAND:-install}"
readonly apply_command="${APPLY_COMMAND:-make install.apply}"
readonly verify_command="${VERIFY_COMMAND:-make install.check}"

function usage {
    printf "%s\n" "Usage: install-repocue.bash <action> [options]"
    printf "%s\n" "Actions: check, check-absent, install-dry-run, install, uninstall-dry-run, uninstall"
    printf "%s\n" "Options: --source PATH --destination PATH --command NAME"
}

function die {
    local message="$1"

    printf "error: %s\n" "${message}" >&2
    exit 1
}

function require_command {
    local command_path

    command_path="$(command -v "${install_command}" 2>/dev/null || true)"
    if [[ -z "${command_path}" || ! -x "${command_path}" ]]; then
        printf "%s\n" "install on Debian with: apt install coreutils" >&2
        die "${install_command} is not an executable command"
    fi
}

function validate_inputs {
    if [[ -z "${ACTION}" ]]; then
        usage >&2
        exit 2
    fi
    if [[ -z "${destination}" || "${destination}" != /* ]]; then
        die "--destination must be an absolute path"
    fi
    if [[ ! "${command_name}" =~ ^[A-Za-z0-9._-]+$ ]]; then
        die "--command contains unsupported characters"
    fi
    case "${ACTION}" in
        install | install-dry-run)
            if [[ -z "${source_path}" ]]; then
                die "--source is required for ${ACTION}"
            fi
            ;;
        check | check-absent | uninstall | uninstall-dry-run)
            ;;
        *)
            die "unsupported action: ${ACTION}"
            ;;
    esac
}

function active_command_path {
    command -v "${command_name}" 2>/dev/null || true
}

function print_current_state {
    local active_path
    local destination_state="missing"

    if [[ -x "${destination}" ]]; then
        destination_state="executable"
    elif [[ -e "${destination}" ]]; then
        destination_state="present but not executable"
    fi
    active_path="$(active_command_path)"
    if [[ -z "${active_path}" ]]; then
        active_path="not found"
    fi

    printf "Destination: %s (%s)\n" "${destination}" "${destination_state}"
    printf "Active command: %s\n" "${active_path}"
}

function check_installation {
    local active_path
    local failed=0

    print_current_state
    active_path="$(active_command_path)"
    if [[ ! -x "${destination}" ]]; then
        printf "%s\n" "FAIL: installed executable is missing"
        failed=1
    fi
    if [[ "${active_path}" != "${destination}" ]]; then
        printf "%s\n" "FAIL: active PATH does not resolve to the managed executable"
        failed=1
    fi
    if [[ "${failed}" -ne 0 ]]; then
        return 1
    fi
    printf "%s\n" "PASS: RepoCue installation is active"
}

function preview_installation {
    print_current_state
    printf "Source: %s\n" "${source_path}"
    printf "Proposed action: install executable at %s\n" "${destination}"
    printf "Apply command: %s\n" "${apply_command}"
    printf "Verify command: %s\n" "${verify_command}"
    printf "%s\n" "No filesystem changes were made."
}

function apply_installation {
    local active_path
    local destination_directory

    require_command
    if [[ ! -x "${source_path}" ]]; then
        die "build output is missing or not executable: ${source_path}"
    fi
    destination_directory="${destination%/*}"
    if [[ -z "${destination_directory}" ]]; then
        destination_directory="/"
    fi
    printf "Creating installation directory: %s\n" "${destination_directory}"
    "${install_command}" -d -m 0755 "${destination_directory}"
    printf "Installing executable: %s\n" "${destination}"
    "${install_command}" -m 0755 "${source_path}" "${destination}"
    [[ -x "${destination}" ]] || die "installation did not produce an executable: ${destination}"
    printf "%s\n" "PASS: RepoCue executable installed"
    active_path="$(active_command_path)"
    if [[ "${active_path}" != "${destination}" ]]; then
        printf "%s\n" "WARNING: active PATH does not resolve to the installed executable"
        printf "Verify after PATH activation: %s\n" "${verify_command}"
    else
        printf "%s\n" "PASS: RepoCue installation is active"
    fi
}

function preview_uninstallation {
    print_current_state
    printf "Proposed action: remove %s\n" "${destination}"
    printf "Apply command: %s\n" "${apply_command}"
    printf "Verify command: %s\n" "${verify_command}"
    printf "%s\n" "No filesystem changes were made."
}

function apply_uninstallation {
    if [[ -e "${destination}" ]]; then
        printf "Removing executable: %s\n" "${destination}"
        rm -f -- "${destination}"
    else
        printf "Executable already absent: %s\n" "${destination}"
    fi
    printf "%s\n" "PASS: managed RepoCue executable is absent"
}

function check_absence {
    print_current_state
    if [[ -e "${destination}" ]]; then
        printf "%s\n" "FAIL: managed RepoCue executable is still present"
        return 1
    fi
    printf "%s\n" "PASS: managed RepoCue executable is absent"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --source)
            [[ $# -ge 2 ]] || die "--source requires a value"
            source_path="$2"
            shift 2
            ;;
        --destination)
            [[ $# -ge 2 ]] || die "--destination requires a value"
            destination="$2"
            shift 2
            ;;
        --command)
            [[ $# -ge 2 ]] || die "--command requires a value"
            command_name="$2"
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

validate_inputs

case "${ACTION}" in
    check)
        check_installation
        ;;
    check-absent)
        check_absence
        ;;
    install-dry-run)
        preview_installation
        ;;
    install)
        apply_installation
        ;;
    uninstall-dry-run)
        preview_uninstallation
        ;;
    uninstall)
        apply_uninstallation
        ;;
esac
