#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK_DIR="$(git rev-parse --show-toplevel)/.git/hooks"

ln -sf "$SCRIPT_DIR/pre-commit" "$HOOK_DIR/pre-commit"
echo "Pre-commit hook installed."
