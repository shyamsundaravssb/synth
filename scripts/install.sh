#!/usr/bin/env bash
set -euo pipefail

# Synth installer — downloads the latest release
# from GitHub and installs to /usr/local/bin/synth
#
# Requires: gh (GitHub CLI), authenticated with
#           your GitHub account (gh auth login)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/shyamsundaravssb/synth/main/scripts/install.sh | bash
#
# Or clone the repo and run:
#   bash scripts/install.sh

REPO="shyamsundaravssb/synth"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="synth"

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

ARCHIVE="synth_${OS}_${ARCH}.tar.gz"

echo "Installing Synth for ${OS}/${ARCH}..."
echo ""

# Check for gh CLI
if ! command -v gh &>/dev/null; then
  echo "error: GitHub CLI (gh) is required" >&2
  echo "Install it from: https://cli.github.com" >&2
  echo "Then run: gh auth login" >&2
  exit 1
fi

# Download latest release
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading $ARCHIVE..."
gh release download --repo "$REPO" \
  --pattern "$ARCHIVE" \
  --pattern "checksums.txt" \
  --dir "$TMPDIR"

# Verify checksum
echo "Verifying checksum..."
cd "$TMPDIR"
if ! sha256sum -c checksums.txt \
     --ignore-missing 2>/dev/null; then
  echo "error: checksum verification failed" >&2
  exit 1
fi

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
