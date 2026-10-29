#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP=$(mktemp -d "${TMPDIR:-/tmp}/aur-installer.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
go_bin=${GO-go}
VERSION=0.2.0
ARCHIVE="agent-usage-reserve_${VERSION}_linux_amd64.tar.gz"

mkdir -p "$TMP/bin" "$TMP/assets" "$TMP/stage"
cat > "$TMP/bin/curl" <<'CURL'
#!/bin/sh
set -eu
url=
out=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift 2 ;;
    -*) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s\n' "$url" >> "$CURL_LOG"
case "$url" in
  https://api.github.com/repos/taejunoh/agent-usage-reserve/releases/latest)
    printf '{"tag_name":"v0.2.0"}\n' > "$out"
    ;;
  *)
    name=${url##*/}
    cp "$ASSET_DIR/$name" "$out"
    ;;
esac
CURL
chmod +x "$TMP/bin/curl"

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

# Build the real binary and package it like a release archive.
make_fixture() {
  (cd "$ROOT" && "$go_bin" build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$TMP/stage/agent-usage-reserve" ./cmd/agent-usage-reserve)
  tar -czf "$TMP/assets/$ARCHIVE" -C "$TMP/stage" agent-usage-reserve
  printf '%s  %s\n' "$(sha256_of "$TMP/assets/$ARCHIVE")" "$ARCHIVE" > "$TMP/assets/checksums.txt"
  cp "$TMP/assets/checksums.txt" "$TMP/good-checksums.txt"
}

# A stand-in for the pre-rename executable.
seed_old() {
  cat > "$1/codex-usage-reserve" <<'OLD'
#!/bin/sh
if [ "${1-}" = "--version" ]; then
  printf '0.1.0\n'
  exit 0
fi
OLD
  chmod +x "$1/codex-usage-reserve"
}

assert() {
  if ! "$@"; then
    echo "assertion failed: $*" >&2
    exit 1
  fi
}

# Fresh sandbox for one case: fake HOME, install dir and agent homes.
new_case() {
  case_dir="$TMP/$1"
  mkdir -p "$case_dir/home" "$case_dir/install" "$case_dir/codex-home" "$case_dir/claude-home"
  export HOME="$case_dir/home"
  export AGENT_USAGE_RESERVE_INSTALL_DIR="$case_dir/install"
  : > "$CURL_LOG"
}

make_fixture
export PATH="$TMP/bin:$PATH"
export CURL_LOG="$TMP/curl.log" ASSET_DIR="$TMP/assets"
export AGENT_USAGE_RESERVE_OS=linux AGENT_USAGE_RESERVE_ARCH=amd64
export AGENT_USAGE_RESERVE_RELEASE_BASE_URL="file://$TMP/assets"
unset AGENT_USAGE_RESERVE_VERSION AGENT_USAGE_RESERVE_CODEX_HOME AGENT_USAGE_RESERVE_CLAUDE_HOME CODEX_HOME CLAUDE_CONFIG_DIR

# README's pipe install path: no arguments, both agents enabled through explicit homes
# (no prompt), and the old executable is removed after the new one is installed.
new_case pipe
seed_old "$case_dir/install"
AGENT_USAGE_RESERVE_CODEX_HOME="$case_dir/codex-home" AGENT_USAGE_RESERVE_CLAUDE_HOME="$case_dir/claude-home" \
  sh -c 'cat "$1" | sh' _ "$ROOT/install.sh" >"$TMP/pipe.out" 2>&1
assert test -x "$case_dir/install/agent-usage-reserve"
assert test "$("$case_dir/install/agent-usage-reserve" --version)" = "$VERSION"
assert grep -q "Installed and enabled: $case_dir/install/agent-usage-reserve" "$TMP/pipe.out"
assert grep -q 'agent-usage-reserve:managed:v1:begin agent=codex' "$case_dir/codex-home/AGENTS.md"
assert grep -q 'agent-usage-reserve:managed:v1:begin agent=claude' "$case_dir/claude-home/CLAUDE.md"
assert test ! -e "$case_dir/install/codex-usage-reserve"
assert grep -q "Removed old codex-usage-reserve: $case_dir/install/codex-usage-reserve" "$TMP/pipe.out"
assert grep -q "/v$VERSION/$ARCHIVE\$" "$CURL_LOG"
assert test ! -e "$case_dir/home/.codex"
assert test ! -e "$case_dir/home/.claude"

# A no-v explicit version is normalized to the v-prefixed release tag; repeat install is idempotent.
: > "$CURL_LOG"
AGENT_USAGE_RESERVE_VERSION=$VERSION AGENT_USAGE_RESERVE_CODEX_HOME="$case_dir/codex-home" AGENT_USAGE_RESERVE_CLAUDE_HOME="$case_dir/claude-home" \
  sh "$ROOT/install.sh" >"$TMP/explicit.out" 2>&1
assert grep -q "/v$VERSION/$ARCHIVE\$" "$CURL_LOG"
assert test "$(wc -l < "$CURL_LOG" | tr -d ' ')" -eq 2
assert grep -q 'Installed and enabled' "$TMP/explicit.out"
assert test "$(grep -c 'managed:v1:begin' "$case_dir/codex-home/AGENTS.md")" -eq 1
assert test ! -e "$case_dir/install/codex-usage-reserve"


# CODEX_HOME / CLAUDE_CONFIG_DIR are read by the binary itself; forwarding them as flags would
# force-select that agent and skip detection of the other one.
assert test -z "$(grep -E 'CODEX_HOME|CLAUDE_CONFIG_DIR' "$ROOT/install.sh" | grep -v 'AGENT_USAGE_RESERVE_')"

# No detectable agent: installed, exit 0, policy not enabled, guidance repeated.
new_case noagent
seed_old "$case_dir/install"
sh "$ROOT/install.sh" >"$TMP/noagent.out" 2>&1 || { cat "$TMP/noagent.out" >&2; echo 'expected exit 0 with no agent' >&2; exit 1; }
assert test -x "$case_dir/install/agent-usage-reserve"
assert grep -q "Installed (policy not enabled yet): $case_dir/install/agent-usage-reserve" "$TMP/noagent.out"
assert grep -q 'Codex or Claude was not found' "$TMP/noagent.out"
assert test ! -e "$case_dir/install/codex-usage-reserve"
assert test ! -e "$case_dir/home/.codex"
assert test ! -e "$case_dir/home/.claude"

# Any other enable failure (a refused home) is propagated; the binary stays installed.
new_case refused
printf 'real\n' > "$case_dir/real-agents.md"
ln -s "$case_dir/real-agents.md" "$case_dir/codex-home/AGENTS.md"
if AGENT_USAGE_RESERVE_CODEX_HOME="$case_dir/codex-home" sh "$ROOT/install.sh" >"$TMP/refused.out" 2>&1; then
  echo 'expected installer to fail when enable refuses the home' >&2
  exit 1
fi
assert test -x "$case_dir/install/agent-usage-reserve"
if grep -q 'Installed and enabled' "$TMP/refused.out"; then echo "unexpected output matched" >&2; exit 1; fi
if grep -q 'policy not enabled yet' "$TMP/refused.out"; then echo "unexpected output matched" >&2; exit 1; fi
assert test "$(cat "$case_dir/real-agents.md")" = real

# A checksum failure must leave existing binaries and instructions untouched and must not enable.
new_case badsum
seed_old "$case_dir/install"
printf 'previous install\n' > "$case_dir/install/agent-usage-reserve"
chmod +x "$case_dir/install/agent-usage-reserve"
cp "$case_dir/install/agent-usage-reserve" "$case_dir/prev-binary"
cp "$case_dir/install/codex-usage-reserve" "$case_dir/prev-old"
printf '0000000000000000000000000000000000000000000000000000000000000000  %s\n' "$ARCHIVE" > "$TMP/assets/checksums.txt"
if AGENT_USAGE_RESERVE_CODEX_HOME="$case_dir/codex-home" AGENT_USAGE_RESERVE_CLAUDE_HOME="$case_dir/claude-home" \
  sh "$ROOT/install.sh" >"$TMP/bad.out" 2>&1; then
  echo 'expected checksum failure' >&2
  exit 1
fi
cmp "$case_dir/prev-binary" "$case_dir/install/agent-usage-reserve"
cmp "$case_dir/prev-old" "$case_dir/install/codex-usage-reserve"
assert test ! -e "$case_dir/codex-home/AGENTS.md"
assert test ! -e "$case_dir/claude-home/CLAUDE.md"
if grep -q 'Removed old' "$TMP/bad.out"; then echo "unexpected output matched" >&2; exit 1; fi
cp "$TMP/good-checksums.txt" "$TMP/assets/checksums.txt"

# A version mismatch (binary reports 0.2.0, tag says 0.9.9) fails the same way.
new_case badver
seed_old "$case_dir/install"
cp "$TMP/assets/$ARCHIVE" "$TMP/assets/agent-usage-reserve_0.9.9_linux_amd64.tar.gz"
printf '%s  agent-usage-reserve_0.9.9_linux_amd64.tar.gz\n' "$(sha256_of "$TMP/assets/$ARCHIVE")" > "$TMP/assets/checksums.txt"
if AGENT_USAGE_RESERVE_VERSION=0.9.9 AGENT_USAGE_RESERVE_CODEX_HOME="$case_dir/codex-home" \
  sh "$ROOT/install.sh" >"$TMP/badver.out" 2>&1; then
  echo 'expected version mismatch failure' >&2
  exit 1
fi
assert test ! -e "$case_dir/install/agent-usage-reserve"
assert test -f "$case_dir/install/codex-usage-reserve"
assert test ! -e "$case_dir/codex-home/AGENTS.md"
cp "$TMP/good-checksums.txt" "$TMP/assets/checksums.txt"

printf 'installer integration checks passed\n'
