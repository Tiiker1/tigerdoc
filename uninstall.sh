#!/bin/sh
# Docker Dashboard uninstaller for Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/uninstall.sh | sudo sh
#
# Options:
#   --dir <path>   Where the binary was installed (default: /usr/local/bin)
#   --help         Show this help

set -eu

BIN_NAME="${BIN_NAME:-docker-dashboard}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
UNIT="${UNIT:-/etc/systemd/system/$BIN_NAME.service}"

say()  { printf '  %s\n' "$*"; }
info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror:\033[0m %b\n' "$*" >&2; exit 1; }

usage() { sed -n '2,7p' "$0" 2>/dev/null || true; exit 0; }

while [ "$#" -gt 0 ]; do
	case "$1" in
		--dir)     INSTALL_DIR="${2:?--dir needs a value}"; shift 2 ;;
		--help|-h) usage ;;
		*)         die "unknown option: $1 (try --help)" ;;
	esac
done

[ "$(id -u)" -eq 0 ] || die "must run as root. Re-run: curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/uninstall.sh | sudo sh"

bin="$INSTALL_DIR/$BIN_NAME"
removed=0

info "Uninstalling $BIN_NAME"

# ---- stop + remove the systemd service -------------------------------------
if command -v systemctl >/dev/null 2>&1 && [ -f "$UNIT" ]; then
	systemctl disable --now "$BIN_NAME" >/dev/null 2>&1 || systemctl stop "$BIN_NAME" >/dev/null 2>&1 || true
	rm -f "$UNIT"
	systemctl daemon-reload >/dev/null 2>&1 || true
	say "removed service unit: $UNIT"
	removed=1
fi

# ---- kill any process still running ----------------------------------------
if command -v pgrep >/dev/null 2>&1 && pgrep -x "$BIN_NAME" >/dev/null 2>&1; then
	warn "$BIN_NAME is still running; stopping it"
	command -v pkill >/dev/null 2>&1 && pkill -x "$BIN_NAME" 2>/dev/null || true
fi

# ---- remove the binary ------------------------------------------------------
if [ -f "$bin" ]; then
	rm -f "$bin"
	say "removed binary: $bin"
	removed=1
elif command -v "$BIN_NAME" >/dev/null 2>&1; then
	other="$(command -v "$BIN_NAME")"
	warn "binary not at $bin but found at $other — removing it"
	rm -f "$other"
	say "removed binary: $other"
	removed=1
fi

if [ "$removed" -eq 0 ]; then
	warn "nothing found to uninstall"
fi

printf '\n\033[1;32mDocker Dashboard removed.\033[0m\n\n'