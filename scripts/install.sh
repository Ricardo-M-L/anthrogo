#!/usr/bin/env sh
# anthrogo install script — downloads the right release tarball for your
# OS/arch and installs the binary to /usr/local/bin (or $ANTHROGO_PREFIX/bin).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Ricardo-M-L/anthrogo/main/scripts/install.sh | sh
#
# Environment overrides:
#   ANTHROGO_VERSION  — install a specific tag (default: latest)
#   ANTHROGO_PREFIX   — install dir prefix (default: /usr/local)
#   ANTHROGO_REPO     — GitHub owner/repo (default: Ricardo-M-L/anthrogo)
#
# Refuses to run as root unless ANTHROGO_PREFIX writes there are required;
# uses sudo to install to /usr/local/bin when needed.

set -eu

REPO="${ANTHROGO_REPO:-Ricardo-M-L/anthrogo}"
PREFIX="${ANTHROGO_PREFIX:-/usr/local}"
BIN_DIR="${PREFIX}/bin"
VERSION="${ANTHROGO_VERSION:-}"

# --- detect platform ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH=x86_64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "anthrogo install: unsupported arch: $ARCH" >&2; exit 1 ;;
esac
case "$OS" in
    darwin|linux) ;;
    *) echo "anthrogo install: unsupported os: $OS (only darwin/linux ship prebuilt; use 'go install' instead)" >&2; exit 1 ;;
esac

# --- check prerequisites ---
for cmd in curl tar; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "anthrogo install: missing required tool: $cmd" >&2
        exit 1
    fi
done

# --- resolve version ---
if [ -z "$VERSION" ]; then
    echo "anthrogo install: resolving latest version..." >&2
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name":' \
        | head -1 \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "anthrogo install: could not resolve latest version from GitHub API" >&2
        echo "  set ANTHROGO_VERSION explicitly to a tag like v0.14.1" >&2
        exit 1
    fi
fi
# strip optional leading 'v' for the filename, GitHub release naming convention
VTAG="$VERSION"
VNUM="${VTAG#v}"
echo "anthrogo install: target version $VTAG" >&2

TARBALL="anthrogo_${VNUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VTAG}/${TARBALL}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VTAG}/checksums.txt"

# --- download to a temp dir ---
TMP=$(mktemp -d -t anthrogo-install-XXXXXX)
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "anthrogo install: downloading $URL" >&2
if ! curl -fsSL -o "$TMP/$TARBALL" "$URL"; then
    echo "anthrogo install: download failed — check that ${TARBALL} exists in the ${VTAG} release" >&2
    exit 1
fi

# --- verify checksum (best-effort: only if checksums.txt is reachable) ---
if curl -fsSL -o "$TMP/checksums.txt" "$CHECKSUMS_URL" 2>/dev/null; then
    if command -v shasum >/dev/null 2>&1; then
        SHA_CMD="shasum -a 256"
    elif command -v sha256sum >/dev/null 2>&1; then
        SHA_CMD="sha256sum"
    else
        SHA_CMD=""
    fi
    if [ -n "$SHA_CMD" ]; then
        EXPECTED=$(grep "  $TARBALL$" "$TMP/checksums.txt" | head -1 | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            ACTUAL=$($SHA_CMD "$TMP/$TARBALL" | awk '{print $1}')
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                echo "anthrogo install: checksum MISMATCH" >&2
                echo "  expected: $EXPECTED" >&2
                echo "  actual:   $ACTUAL" >&2
                exit 1
            fi
            echo "anthrogo install: checksum verified" >&2
        fi
    fi
fi

# --- extract ---
echo "anthrogo install: extracting" >&2
tar -xzf "$TMP/$TARBALL" -C "$TMP" anthrogo

if [ ! -f "$TMP/anthrogo" ]; then
    echo "anthrogo install: tarball did not contain 'anthrogo' binary at top level" >&2
    exit 1
fi
chmod +x "$TMP/anthrogo"

# --- install ---
mkdir -p "$BIN_DIR" 2>/dev/null || true
if [ -w "$BIN_DIR" ]; then
    mv "$TMP/anthrogo" "$BIN_DIR/anthrogo"
elif command -v sudo >/dev/null 2>&1; then
    echo "anthrogo install: $BIN_DIR not writable; using sudo" >&2
    sudo mv "$TMP/anthrogo" "$BIN_DIR/anthrogo"
else
    echo "anthrogo install: cannot write to $BIN_DIR (no sudo). Either:" >&2
    echo "  - rerun with ANTHROGO_PREFIX=\$HOME/.local" >&2
    echo "  - copy $TMP/anthrogo manually to a directory on your PATH" >&2
    exit 1
fi

# --- macOS quarantine ---
if [ "$OS" = "darwin" ]; then
    xattr -dr com.apple.quarantine "$BIN_DIR/anthrogo" 2>/dev/null || true
fi

# --- verify ---
INSTALLED_VERSION=$("$BIN_DIR/anthrogo" version 2>&1 || echo "verify failed")
echo "" >&2
echo "anthrogo install: done." >&2
echo "  path:    $BIN_DIR/anthrogo" >&2
echo "  version: $INSTALLED_VERSION" >&2
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$BIN_DIR"; then
    echo "  note:    $BIN_DIR is not on your PATH yet" >&2
    echo "           add 'export PATH=\"$BIN_DIR:\$PATH\"' to ~/.zshrc or ~/.bashrc" >&2
fi
