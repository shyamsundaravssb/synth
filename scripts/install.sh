#!/usr/bin/env bash
set -euo pipefail

# Synth installer — downloads the latest release
# from GitHub and installs to /usr/local/bin/synth
#
# Requires: curl, sha256sum (Linux) or
#           shasum (macOS) — both standard
#
# Usage (one-liner):
#   curl -fsSL https://raw.githubusercontent.com/\
# shyamsundaravssb/synth/main/scripts/install.sh \
#   | bash
#
# To install a specific version:
#   SYNTH_VERSION=v0.1.0 curl -fsSL ... | bash
#
# Or clone the repo and run:
#   bash scripts/install.sh

REPO="shyamsundaravssb/synth"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="synth"

TAG="${SYNTH_VERSION:-}"
# If SYNTH_VERSION env var is set, installs
# that specific version. Otherwise installs
# the latest stable release.

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)  OS="linux"  ;;
  darwin*) OS="darwin" ;;
  *)
    echo "error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Detect checksum command (sha256sum on Linux,
# shasum on macOS)
if command -v sha256sum &>/dev/null; then
  SHASUM_CMD="sha256sum"
elif command -v shasum &>/dev/null; then
  SHASUM_CMD="shasum -a 256"
else
  echo "error: no sha256 tool found" >&2
  echo "Install coreutils or shasum" >&2
  exit 1
fi

ARCHIVE="synth_${OS}_${ARCH}.tar.gz"

# Download latest release
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Determine version to install
if [ -n "$TAG" ]; then
  VERSION="$TAG"
else
  echo "Fetching latest release version..."
  VERSION=$(curl -fsSL \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "error: could not determine latest release" >&2
    exit 1
  fi
fi

DOWNLOAD_BASE="https://github.com/${REPO}/releases/download/${VERSION}"

echo "Installing Synth ${VERSION} for ${OS}/${ARCH}..."
echo ""

echo "Downloading ${ARCHIVE}..."
curl -fsSL \
  "${DOWNLOAD_BASE}/${ARCHIVE}" \
  -o "${TMPDIR}/${ARCHIVE}"

echo "Downloading checksums..."
curl -fsSL \
  "${DOWNLOAD_BASE}/checksums.txt" \
  -o "${TMPDIR}/checksums.txt"

# Verify checksum
echo "Verifying checksum..."
cd "$TMPDIR"
# Extract checksum for our specific archive
EXPECTED=$(grep "$ARCHIVE" checksums.txt \
           | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "error: checksum not found for $ARCHIVE" >&2
  exit 1
fi

# Compute actual checksum
ACTUAL=$($SHASUM_CMD "$ARCHIVE" | awk '{print $1}')

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "error: checksum verification failed" >&2
  echo "  expected: $EXPECTED" >&2
  echo "  actual:   $ACTUAL" >&2
  exit 1
fi

echo "Checksum verified."

# Extract and install
echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
tar -xzf "$ARCHIVE"
sudo install -m 755 synth "$INSTALL_DIR/$BINARY_NAME"

echo ""
echo "✓ Synth installed successfully"
echo ""
echo "Get started:"
echo "  cd <your-git-repo>"
echo "  synth init"
echo "  synth model download   # for semantic search"
echo "  synth daemon start"
