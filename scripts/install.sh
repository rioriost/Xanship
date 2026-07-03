#!/usr/bin/env sh
set -eu

repo="rioriost/Xanship"
version="${XANSHIP_VERSION:-${1:-latest}}"

if [ "${INSTALL_DIR:-}" ]; then
  install_dir="$INSTALL_DIR"
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  install_dir="/usr/local/bin"
elif [ "${HOME:-}" ]; then
  install_dir="$HOME/.local/bin"
else
  echo "INSTALL_DIR is not set and HOME is unavailable" >&2
  exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin|linux) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64) arch="amd64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
fi

case "$version" in
  v*) tag="$version"; version_no_v="${version#v}" ;;
  *) tag="v$version"; version_no_v="$version" ;;
esac

asset="xanship_${version_no_v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$repo/releases/download/$tag"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

curl -fsSL "$base_url/$asset" -o "$tmpdir/$asset"
curl -fsSL "$base_url/checksums.txt" -o "$tmpdir/checksums.txt"

if command -v shasum >/dev/null 2>&1; then
  (cd "$tmpdir" && grep "  $asset\$" checksums.txt | shasum -a 256 -c -)
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmpdir" && grep "  $asset\$" checksums.txt | sha256sum -c -)
else
  echo "warning: shasum/sha256sum not found; skipping checksum verification" >&2
fi

tar -xzf "$tmpdir/$asset" -C "$tmpdir"
mkdir -p "$install_dir"
install "$tmpdir/xanship" "$install_dir/xanship"

echo "installed xanship $tag to $install_dir/xanship"

case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) echo "note: add $install_dir to PATH to run xanship without a full path" ;;
esac
