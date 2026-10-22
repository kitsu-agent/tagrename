#!/usr/bin/env bash
set -euo pipefail

REPO="${TAGRENAME_REPO:-kitsu-agent/tagrename}"
BIN="tagrename"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

url="https://github.com/${REPO}/releases/latest/download/${BIN}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${url}"
curl -fsSL "$url" -o "$tmp/$BIN.tar.gz"
tar -xzf "$tmp/$BIN.tar.gz" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN"
echo "Installed $BIN to $INSTALL_DIR/$BIN"

if ! command -v ffprobe >/dev/null 2>&1; then
  echo "warning: ffprobe not found. Install ffmpeg (e.g. 'sudo apt install ffmpeg' or 'brew install ffmpeg')."
fi

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "note: add $INSTALL_DIR to your PATH." ;;
esac
