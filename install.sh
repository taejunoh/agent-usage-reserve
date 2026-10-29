#!/bin/sh
set -eu

latest=
tmp=
cleanup() {
  if [ -n "$latest" ]; then rm -f "$latest"; fi
  if [ -n "$tmp" ]; then rm -rf "$tmp"; fi
}
trap cleanup EXIT

REPO=taejunoh/agent-usage-reserve
API_URL="https://api.github.com/repos/$REPO/releases/latest"
version=${AGENT_USAGE_RESERVE_VERSION-}

if [ -z "$version" ]; then
  latest=$(mktemp "${TMPDIR:-/tmp}/agent-usage-reserve-latest.XXXXXX")
  curl -fsSL "$API_URL" -o "$latest"
  tag=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$latest" | head -n 1)
else
  tag=$version
fi
case "$tag" in v*) version=${tag#v} ;; *) version=$tag; tag="v$version" ;; esac
if ! printf '%s\n' "$tag" | grep -Eq '^v?[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Invalid release version: $tag" >&2
  exit 1
fi

os=${AGENT_USAGE_RESERVE_OS-$(uname -s | tr '[:upper:]' '[:lower:]')}
case "$os" in darwin|linux) ;; *) echo "Unsupported operating system: $os" >&2; exit 1 ;; esac
arch=${AGENT_USAGE_RESERVE_ARCH-$(uname -m)}
case "$arch" in x86_64|x64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; esac
case "$arch" in amd64|arm64) ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac

archive="agent-usage-reserve_${version}_${os}_${arch}.tar.gz"
base=${AGENT_USAGE_RESERVE_RELEASE_BASE_URL-"https://github.com/$REPO/releases/download"}/$tag
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-usage-reserve-install.XXXXXX")
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$tmp/checksums.txt")
if [ "$(printf '%s\n' "$expected" | wc -l | tr -d ' ')" -ne 1 ] || ! printf '%s\n' "$expected" | grep -Eq '^[[:xdigit:]]{64}$'; then
  echo "No unique SHA-256 checksum found for $archive" >&2
  exit 1
fi
if command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
elif command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  echo 'Install shasum or sha256sum to verify the download.' >&2
  exit 1
fi
if [ "$actual" != "$expected" ]; then
  echo "SHA-256 mismatch for $archive" >&2
  exit 1
fi

mkdir "$tmp/unpacked"
tar -xzf "$tmp/$archive" -C "$tmp/unpacked" agent-usage-reserve
staged="$tmp/unpacked/agent-usage-reserve"
if [ ! -f "$staged" ] || [ "$("$staged" --version)" != "$version" ]; then
  echo "Release binary version does not match tag $tag" >&2
  exit 1
fi

install_dir=${AGENT_USAGE_RESERVE_INSTALL_DIR-"$HOME/.local/bin"}
mkdir -p "$install_dir"
target="$install_dir/agent-usage-reserve"
chmod 755 "$staged"
mv -f "$staged" "$target"

# The old executable cannot see the new marker and would add a second policy block.
old="$install_dir/codex-usage-reserve"
if [ -f "$old" ] && [ ! -L "$old" ]; then
  rm -f "$old"
  printf 'Removed old codex-usage-reserve: %s\n' "$old"
fi

# The binary reads the agents' own home variables itself; passing them as flags would
# force-select that agent and skip detection of the other one.
set -- enable
if [ -n "${AGENT_USAGE_RESERVE_CODEX_HOME-}" ]; then
  set -- "$@" --codex-home "$AGENT_USAGE_RESERVE_CODEX_HOME"
fi
if [ -n "${AGENT_USAGE_RESERVE_CLAUDE_HOME-}" ]; then
  set -- "$@" --claude-home "$AGENT_USAGE_RESERVE_CLAUDE_HOME"
fi
# Under curl | sh, stdin is the script; prompt through the terminal when there is one.
status=0
if ( : </dev/tty ) 2>/dev/null; then
  "$target" "$@" </dev/tty || status=$?
else
  "$target" "$@" || status=$?
fi
case "$status" in
  0) printf 'Installed and enabled: %s\n' "$target" ;;
  3) printf 'Installed (policy not enabled yet): %s\n' "$target" ;;
  *) exit "$status" ;;
esac
case ":${PATH-}:" in *":$install_dir:"*) ;; *) printf 'Add this to PATH: export PATH="%s:$PATH"\n' "$install_dir" ;; esac
