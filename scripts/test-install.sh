#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP=$(mktemp -d "${TMPDIR:-/tmp}/cur-installer.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM

mkdir -p "$TMP/bin" "$TMP/assets" "$TMP/codex-home" "$TMP/install"
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
  https://api.github.com/repos/taejunoh/codex-usage-reserve/releases/latest)
    printf '{"tag_name":"v0.1.0"}\n' > "$out"
    ;;
  *)
    name=${url##*/}
    cp "$ASSET_DIR/$name" "$out"
    ;;
esac
CURL
chmod +x "$TMP/bin/curl"

make_fixture() {
  cat > "$TMP/codex-usage-reserve" <<'BIN'
#!/bin/sh
if [ "${1-}" = "--version" ]; then
  printf '0.1.0\n'
  exit 0
fi
printf '%s\n' "$*" >> "$ENABLE_LOG"
printf 'CODEX_HOME=%s\n' "${CODEX_HOME-}" >> "$ENABLE_LOG"
BIN
  chmod +x "$TMP/codex-usage-reserve"
  tar -czf "$TMP/assets/codex-usage-reserve_0.1.0_linux_amd64.tar.gz" -C "$TMP" codex-usage-reserve
  (cd "$TMP/assets" && shasum -a 256 codex-usage-reserve_0.1.0_linux_amd64.tar.gz > checksums.txt)
}

assert() {
  if ! "$@"; then
    echo "assertion failed: $*" >&2
    exit 1
  fi
}

make_fixture
export PATH="$TMP/bin:$PATH"
export CURL_LOG="$TMP/curl.log" ASSET_DIR="$TMP/assets"
export ENABLE_LOG="$TMP/enable.log"
export CODEX_USAGE_RESERVE_CODEX_HOME="$TMP/codex-home"
export CODEX_USAGE_RESERVE_OS=linux CODEX_USAGE_RESERVE_ARCH=amd64
export CODEX_USAGE_RESERVE_RELEASE_BASE_URL="file://$TMP/assets"
export CODEX_USAGE_RESERVE_INSTALL_DIR="$TMP/install"

# README's pipe install path must work without arguments and enable after install.
cat "$ROOT/install.sh" | sh
assert test -x "$TMP/install/codex-usage-reserve"
assert grep -q '^enable --codex-home ' "$ENABLE_LOG"
assert grep -q '/v0.1.0/codex-usage-reserve_0.1.0_linux_amd64.tar.gz$' "$CURL_LOG"

# A no-v explicit version is normalized to the actual v-prefixed release tag.
: > "$CURL_LOG"
: > "$ENABLE_LOG"
export CODEX_USAGE_RESERVE_VERSION=0.1.0
sh "$ROOT/install.sh" >"$TMP/explicit.out"
assert grep -q '/v0.1.0/codex-usage-reserve_0.1.0_linux_amd64.tar.gz$' "$CURL_LOG"
assert test "$(wc -l < "$CURL_LOG" | tr -d ' ')" -eq 2
assert grep -q '^enable --codex-home ' "$ENABLE_LOG"
unset CODEX_USAGE_RESERVE_VERSION

# A checksum failure must leave an existing binary intact and must not enable.
: > "$ENABLE_LOG"
printf 'old executable\n' > "$TMP/install/codex-usage-reserve"
chmod +x "$TMP/install/codex-usage-reserve"
cp "$TMP/install/codex-usage-reserve" "$TMP/old-binary"
cp "$TMP/assets/checksums.txt" "$TMP/good-checksums.txt"
printf '0000000000000000000000000000000000000000000000000000000000000000  codex-usage-reserve_0.1.0_linux_amd64.tar.gz\n' > "$TMP/assets/checksums.txt"
if sh "$ROOT/install.sh" >"$TMP/bad.out" 2>&1; then
  echo 'expected checksum failure' >&2
  exit 1
fi
cmp "$TMP/old-binary" "$TMP/install/codex-usage-reserve"
assert test ! -s "$ENABLE_LOG"

printf 'installer integration checks passed\n'
