#!/usr/bin/env sh
# Install the latest mnemo release binary — no Go required.
#   curl -fsSL https://raw.githubusercontent.com/JaraEsequiel/mnemo/main/scripts/get.sh | sh
set -e

REPO="JaraEsequiel/mnemo"
BIN_DIR="${MNEMO_BIN_DIR:-$HOME/.local/bin}"

os=$(uname -s)
arch=$(uname -m)
case "$os" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

echo "Resolving latest mnemo release…"
tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d '"' -f4)
if [ -z "$tag" ]; then
  echo "could not find a release; install with Go instead:" >&2
  echo "  go install github.com/$REPO/cmd/mnemo@latest" >&2
  exit 1
fi
ver=${tag#v}
url="https://github.com/$REPO/releases/download/$tag/mnemo_${ver}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/mnemo.tar.gz"
tar -xzf "$tmp/mnemo.tar.gz" -C "$tmp"
mkdir -p "$BIN_DIR"
mv "$tmp/mnemo" "$BIN_DIR/mnemo"
chmod +x "$BIN_DIR/mnemo"

echo "✓ installed mnemo $ver to $BIN_DIR/mnemo"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "! add $BIN_DIR to your PATH: export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
echo "Next: run  mnemo setup"
