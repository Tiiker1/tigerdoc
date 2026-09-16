#!/bin/sh
# Docker Dashboard updater for Linux.
#
# Re-runs the installer and reuses the configuration detected from the
# existing systemd unit (port, allowed subnets, stop timeout), so an update
# keeps your setup. Extra arguments are forwarded to the installer and
# override the detected configuration.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/update.sh | sudo sh
#
# Example (keep current port, override subnets):
#   curl -fsSL .../update.sh | sudo sh -s -- --subnets 10.0.0.0/8

set -eu

REPO="${REPO:-Tiiker1/tigerdoc}"
BRANCH="${BRANCH:-main}"
BIN_NAME="${BIN_NAME:-docker-dashboard}"
UNIT="${UNIT:-/etc/systemd/system/$BIN_NAME.service}"
RAW_BASE="${RAW_BASE:-https://raw.githubusercontent.com/$REPO/$BRANCH}"

say()  { printf '  %s\n' "$*"; }
info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31merror:\033[0m %b\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || {
	if command -v sudo >/dev/null 2>&1; then
		exec sudo "$0" "$@"
	fi
	die "must run as root. Re-run: curl -fsSL $RAW_BASE/update.sh | sudo sh"
}

# ---- detect existing configuration ------------------------------------------
PORT=""
SUBNETS=""
STOPT=""
if [ -f "$UNIT" ]; then
	PORT="$(sed -n 's/^Environment=DASHBOARD_ADDR=://p' "$UNIT" | head -n1 | tr -d '\r')"
	SUBNETS="$(sed -n 's/^Environment=ALLOWED_SUBNETS=//p' "$UNIT" | head -n1 | tr -d '\r')"
	STOPT="$(sed -n 's/^Environment=STOP_TIMEOUT=//p' "$UNIT" | head -n1 | tr -d '\r')"
	if [ -n "$PORT$SUBNETS$STOPT" ]; then
		say "reusing detected config: port=${PORT:-default} subnets=${SUBNETS:-default} stop-timeout=${STOPT:-default}"
	fi
else
	say "no service unit found; using installer defaults"
fi

# ---- fetch + run the installer ----------------------------------------------
tmp="$(mktemp -d 2>/dev/null || mktemp -d -t dashboard-update)"
trap 'rm -rf "$tmp"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
	dl() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	dl() { wget -qO "$2" "$1"; }
else
	die "curl or wget is required"
fi
dl "$RAW_BASE/install.sh" "$tmp/install.sh"

info "Updating $BIN_NAME from $BRANCH branch"
env PORT="$PORT" ALLOWED_SUBNETS="$SUBNETS" STOP_TIMEOUT="$STOPT" sh "$tmp/install.sh" "$@"