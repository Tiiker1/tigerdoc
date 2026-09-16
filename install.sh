#!/bin/sh
# Docker Dashboard installer for Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Tiiker1/tigerdoc/main/install.sh | sudo sh
#
# By default it installs the binary committed to the main branch (dist/),
# which GitHub Actions rebuilds automatically on every push -> no versions
# to pick. Use --release or --version to install from a tagged release instead.
#
# Options (flags or env vars):
#   --release           Install from the latest GitHub release
#   --version <tag>     Install from a specific release tag, e.g. v1.0.0
#   --port <n>          Port for the dashboard (default: 8080)
#   --subnets <cidrs>   Comma-separated allowed CIDRs (default: private ranges)
#   --dir <path>        Install directory (default: /usr/local/bin)
#   --no-systemd        Install the binary only, do not create a service
#   --help              Show this help
#
# Examples:
#   curl -fsSL .../install.sh | sudo sh -s -- --port 9000
#   curl -fsSL .../install.sh | sudo sh -s -- --subnets 192.168.1.0/24

set -eu

REPO="${REPO:-Tiiker1/tigerdoc}"
BRANCH="${BRANCH:-main}"
BIN_NAME="${BIN_NAME:-docker-dashboard}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
PORT="${PORT:-8080}"
ALLOWED_SUBNETS="${ALLOWED_SUBNETS:-}"
STOP_TIMEOUT="${STOP_TIMEOUT:-}"
VERSION="${VERSION:-}"
SOURCE="${SOURCE:-branch}"   # branch | release
WITH_SYSTEMD="${WITH_SYSTEMD:-auto}"
RAW_BASE="${RAW_BASE:-https://raw.githubusercontent.com/$REPO/$BRANCH}"

say()  { printf '  %s\n' "$*"; }
info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror:\033[0m %b\n' "$*" >&2; exit 1; }

usage() {
	sed -n '2,20p' "$0" 2>/dev/null || true
	exit 0
}

# ---- parse flags ----------------------------------------------------------
while [ $# -gt 0 ]; do
	case "$1" in
		--release)    SOURCE="release"; shift ;;
		--version)    VERSION="${2:?--version needs a value}"; SOURCE="release"; shift 2 ;;
		--port)       PORT="${2:?--port needs a value}"; shift 2 ;;
		--subnets)    ALLOWED_SUBNETS="${2:?--subnets needs a value}"; shift 2 ;;
		--dir)        INSTALL_DIR="${2:?--dir needs a value}"; shift 2 ;;
		--no-systemd) WITH_SYSTEMD="no"; shift ;;
		--systemd)    WITH_SYSTEMD="yes"; shift ;;
		--help|-h)    usage ;;
		*)            die "unknown option: $1 (try --help)" ;;
	esac
done

# ---- must be root ---------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
	if [ -r "$0" ] && command -v sudo >/dev/null 2>&1; then
		exec sudo "$0" "$@"
	fi
	die "must run as root. Re-run: curl -fsSL $RAW_BASE/install.sh | sudo sh"
fi

# ---- platform detection ---------------------------------------------------
os="$(uname -s)"
[ "$os" = "Linux" ] || die "this installer supports Linux only (detected: $os)"

case "$(uname -m)" in
	x86_64|amd64)        arch="amd64" ;;
	aarch64|arm64)       arch="arm64" ;;
	armv7l|armv7|armhf)  arch="armv7" ;;
	*)                   die "unsupported architecture: $(uname -m)" ;;
esac
asset="$BIN_NAME-linux-$arch"

# ---- download helper ------------------------------------------------------
if command -v curl >/dev/null 2>&1; then
	dl() { curl -fsSL "$1" -o "$2"; }
	fetch_text() { curl -fsSL "$1"; }
	fetch_redirect() { curl -fsSLI -o /dev/null -w '%{url_effective}' "$1"; }
elif command -v wget >/dev/null 2>&1; then
	dl() { wget -qO "$2" "$1"; }
	fetch_text() { wget -qO- "$1"; }
	fetch_redirect() { wget -qS --spider --max-redirect=0 "$1" 2>&1 | sed -n 's/.*[Ll]ocation: //p' | head -n1 | tr -d '\r'; }
else
	die "curl or wget is required"
fi

# ---- resolve what to download ---------------------------------------------
case "$SOURCE" in
	release)
		if [ -z "$VERSION" ]; then
			info "Resolving latest release of $REPO"
			tag="$(fetch_redirect "https://github.com/$REPO/releases/latest" 2>/dev/null | sed 's#.*/tag/##; s#[/ ]*$##')"
			case "$tag" in
				""|*releases/latest*|*"$REPO"*)
					tag="$(fetch_text "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
						| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
					;;
			esac
			[ -n "$tag" ] && [ "$tag" != "latest" ] \
				|| die "no release found for $REPO. Create one with:
    git tag v1.0.0
    git push origin main --tags"
		else
			tag="$VERSION"
			case "$tag" in v*) ;; *) tag="v$tag" ;; esac
		fi
		base="https://github.com/$REPO/releases/download/$tag"
		info "Installing $BIN_NAME from release $tag ($arch)"
		;;

	*)
		base="$RAW_BASE/dist"
		info "Installing $BIN_NAME from $BRANCH branch ($arch)"
		;;
esac

# ---- download + verify ----------------------------------------------------
tmp="$(mktemp -d 2>/dev/null || mktemp -d -t dashboard)"
trap 'rm -rf "$tmp"' EXIT INT TERM

if ! dl "$base/$asset" "$tmp/$asset"; then
	case "$SOURCE" in
		release) die "download failed: $base/$asset" ;;
		*) die "no binaries found on the $BRANCH branch yet.
First push to $BRANCH triggers a build by GitHub Actions which commits
dist/ automatically. If you just pushed, wait a minute and re-run this
installer. (Install from a release instead with --release.)" ;;
	esac
fi

if dl "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
	if command -v sha256sum >/dev/null 2>&1; then
		actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
	elif command -v shasum >/dev/null 2>&1; then
		actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
	else
		actual=""
	fi
	expected="$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/checksums.txt")"
	if [ -n "$actual" ] && [ -n "$expected" ]; then
		[ "$actual" = "$expected" ] || die "checksum mismatch for $asset"
		say "checksum verified"
	else
		warn "could not verify checksum"
	fi
fi

# ---- install binary -------------------------------------------------------
mkdir -p "$INSTALL_DIR"
if command -v install >/dev/null 2>&1; then
	install -m 0755 "$tmp/$asset" "$INSTALL_DIR/$BIN_NAME"
else
	cp "$tmp/$asset" "$INSTALL_DIR/$BIN_NAME"
	chmod 0755 "$INSTALL_DIR/$BIN_NAME"
fi
say "installed $INSTALL_DIR/$BIN_NAME"

# ---- systemd service ------------------------------------------------------
have_systemd="no"
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	have_systemd="yes"
fi

if [ "$WITH_SYSTEMD" = "yes" ] && [ "$have_systemd" = "no" ]; then
	warn "systemd not detected; skipping service setup"
fi

if [ "$WITH_SYSTEMD" != "no" ] && [ "$have_systemd" = "yes" ]; then
	unit="/etc/systemd/system/$BIN_NAME.service"
	{
		echo "[Unit]"
		echo "Description=Docker Dashboard (LAN-only)"
		echo "After=network-online.target docker.service"
		echo "Wants=network-online.target"
		echo "Requires=docker.service"
		echo
		echo "[Service]"
		echo "Type=simple"
		echo "ExecStart=$INSTALL_DIR/$BIN_NAME"
		echo "Environment=DASHBOARD_ADDR=:$PORT"
		[ -n "$ALLOWED_SUBNETS" ] && echo "Environment=ALLOWED_SUBNETS=$ALLOWED_SUBNETS"
		[ -n "$STOP_TIMEOUT" ] && echo "Environment=STOP_TIMEOUT=$STOP_TIMEOUT"
		echo "Restart=on-failure"
		echo "RestartSec=3"
		echo "NoNewPrivileges=true"
		echo "ProtectSystem=full"
		echo "ProtectHome=true"
		echo "PrivateTmp=true"
		echo
		echo "[Install]"
		echo "WantedBy=multi-user.target"
	} > "$unit"

	systemctl daemon-reload
	systemctl enable --now "$BIN_NAME" >/dev/null 2>&1 || systemctl restart "$BIN_NAME"
	say "service enabled: $unit"
	say "status: systemctl status $BIN_NAME   logs: journalctl -u $BIN_NAME -f"
else
	say "start it with: $INSTALL_DIR/$BIN_NAME"
fi

# ---- done -----------------------------------------------------------------
ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$ip" ] || ip="<server-ip>"

printf '\n\033[1;32mDocker Dashboard installed.\033[0m\n\n'
printf '  Open:  http://%s:%s\n' "$ip" "$PORT"
printf '  LAN only: public IPs are rejected by default (403).\n'
printf '  Do not port-forward this port on your router/firewall.\n\n'