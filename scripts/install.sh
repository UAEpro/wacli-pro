#!/usr/bin/env bash
#
# wacli-pro installer
#
#   curl -fsSL https://raw.githubusercontent.com/UAEpro/wacli-pro/main/scripts/install.sh | bash
#
# Downloads the latest release binary for your platform and installs it to
# /usr/local/bin (or ~/.local/bin if /usr/local/bin is not writable).
# Override the destination with WACLI_PRO_INSTALL_DIR, or pin a version with
# WACLI_PRO_VERSION (e.g. WACLI_PRO_VERSION=v1.3.0).

set -euo pipefail

REPO="UAEpro/wacli-pro"
BINARY="wacli-pro"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin)
    asset="wacli-pro-macos-universal.tar.gz"
    ;;
  Linux)
    case "$arch" in
      x86_64 | amd64) asset="wacli-pro-linux-amd64.tar.gz" ;;
      aarch64 | arm64) asset="wacli-pro-linux-arm64.tar.gz" ;;
      *)
        echo "error: unsupported Linux architecture: $arch" >&2
        exit 1
        ;;
    esac
    ;;
  *)
    echo "error: unsupported OS: $os" >&2
    echo "On Windows, download wacli-pro-windows-amd64.zip from:" >&2
    echo "  https://github.com/$REPO/releases/latest" >&2
    exit 1
    ;;
esac

version="${WACLI_PRO_VERSION:-latest}"
if [ "$version" = "latest" ]; then
  url="https://github.com/$REPO/releases/latest/download/$asset"
else
  url="https://github.com/$REPO/releases/download/$version/$asset"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading $asset ($version)..."
curl -fsSL "$url" -o "$tmpdir/$asset"
tar -xzf "$tmpdir/$asset" -C "$tmpdir"

if [ ! -f "$tmpdir/$BINARY" ]; then
  echo "error: $BINARY not found in archive" >&2
  exit 1
fi

install_dir="${WACLI_PRO_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -w /usr/local/bin ]; then
    install_dir=/usr/local/bin
  else
    install_dir="$HOME/.local/bin"
  fi
fi
mkdir -p "$install_dir"

install -m 0755 "$tmpdir/$BINARY" "$install_dir/$BINARY"
echo "Installed $BINARY to $install_dir/$BINARY"

if ! command -v "$BINARY" >/dev/null 2>&1; then
  echo
  echo "note: $install_dir is not on your PATH. Add it with:"
  echo "  export PATH=\"$install_dir:\$PATH\""
fi

"$install_dir/$BINARY" --version || true
echo
echo "Get started with: $BINARY auth"
