#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -n "$(gofmt -l cmd internal)" ]]; then
  echo "gofmt required:" >&2
  gofmt -l cmd internal >&2
  exit 1
fi

go test ./...
go vet ./...
sh -n scripts/install.sh

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir" ./xanship' EXIT INT TERM

go build -trimpath -ldflags="-X github.com/rioriost/Xanship/internal/xanship.version=release-check" ./cmd/xanship
version="$(./xanship version)"
if [[ "$version" != "release-check" ]]; then
  echo "version ldflag check failed: $version" >&2
  exit 1
fi

for os in darwin linux; do
  for arch in amd64 arm64; do
    out="xanship_0.0.0_${os}_${arch}"
    mkdir -p "$tmpdir/$out/docs"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w -X github.com/rioriost/Xanship/internal/xanship.version=0.0.0" \
      -o "$tmpdir/$out/xanship" \
      ./cmd/xanship
    cp README.md README.ja.md LICENSE "$tmpdir/$out/"
    cp docs/tested.md "$tmpdir/$out/docs/tested.md"
    tar -C "$tmpdir/$out" -czf "$tmpdir/${out}.tar.gz" .
  done
done

(cd "$tmpdir" && shasum -a 256 *.tar.gz > checksums.txt)
