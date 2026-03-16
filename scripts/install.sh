#!/bin/sh
# Install phctl — one-line installer for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pidginhost/phctl/main/scripts/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR  — override install directory (default: /usr/local/bin or ~/.local/bin)
#   VERSION      — install a specific version (default: latest)

set -eu

REPO="pidginhost/phctl"
GITHUB_API="https://api.github.com/repos/${REPO}/releases"

main() {
    need_cmd curl
    need_cmd uname

    os="$(detect_os)"
    arch="$(detect_arch)"
    version="$(resolve_version)"
    asset="phctl-${os}-${arch}"
    install_dir="$(resolve_install_dir)"

    printf "Installing phctl %s (%s/%s) to %s\n" "$version" "$os" "$arch" "$install_dir"

    download_url="https://github.com/${REPO}/releases/download/${version}/${asset}"

    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' EXIT

    printf "Downloading %s...\n" "$download_url"
    curl -fsSL -o "$tmp" "$download_url"
    chmod +x "$tmp"

    mkdir -p "$install_dir"
    mv "$tmp" "${install_dir}/phctl"
    trap - EXIT

    printf "phctl %s installed to %s/phctl\n" "$version" "$install_dir"

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$install_dir"; then
        printf "\nNote: %s is not in your PATH. Add it with:\n" "$install_dir"
        printf "  export PATH=\"%s:\$PATH\"\n" "$install_dir"
    fi
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            err "Windows is not supported by this installer. Download from https://github.com/${REPO}/releases" ;;
        *) err "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) err "Unsupported architecture: $(uname -m)" ;;
    esac
}

resolve_version() {
    if [ -n "${VERSION:-}" ]; then
        echo "$VERSION"
        return
    fi
    v="$(curl -fsSL "${GITHUB_API}/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')"
    if [ -z "$v" ]; then
        err "Could not determine latest version. Set VERSION explicitly."
    fi
    echo "$v"
}

resolve_install_dir() {
    if [ -n "${INSTALL_DIR:-}" ]; then
        echo "$INSTALL_DIR"
        return
    fi
    if [ -w /usr/local/bin ]; then
        echo "/usr/local/bin"
    else
        echo "${HOME}/.local/bin"
    fi
}

need_cmd() {
    if ! command -v "$1" > /dev/null 2>&1; then
        err "Required command not found: $1"
    fi
}

err() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

main
