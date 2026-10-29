#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 VERSION (for example: 0.2.0)" >&2
  exit 2
fi
version=${1#v}
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Invalid version: $1" >&2
  exit 2
fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${AGENT_USAGE_RESERVE_DIST_DIR-"$root/dist"}
go_bin=${GO-go}
mkdir -p "$out"
rm -f "$out"/agent-usage-reserve_*.tar.gz "$out/checksums.txt"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-usage-reserve-build.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
cd "$root"

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  name="agent-usage-reserve_${version}_${os}_${arch}"
  binary="$tmp/$name/agent-usage-reserve"
  suffix=
  if [ "$os" = windows ]; then
    suffix=.exe
    binary="$tmp/$name/agent-usage-reserve.exe"
  fi
  mkdir -p "$tmp/$name"
  cp "$root/LICENSE" "$tmp/$name/LICENSE"
  printf 'Building %s/%s\n' "$os" "$arch"
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags "-s -w -X main.version=$version" -o "$binary" ./cmd/agent-usage-reserve
  COPYFILE_DISABLE=1 tar -czf "$out/${name}.tar.gz" -C "$tmp/$name" "agent-usage-reserve$suffix" LICENSE
done

cd "$out"
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 agent-usage-reserve_*.tar.gz > checksums.txt
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum agent-usage-reserve_*.tar.gz > checksums.txt
else
  echo 'Install shasum or sha256sum to create release checksums.' >&2
  exit 1
fi
printf 'Release assets written to %s\n' "$out"
