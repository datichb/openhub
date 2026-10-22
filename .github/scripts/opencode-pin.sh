#!/usr/bin/env bash
# Checks the opencode native packages of a version against the npm registry and
# prints what to record (read only: nothing in the repository is modified).
#
#   opencode-pin.sh [--darwin] <version>
#
# For each Linux target (linux-x64, linux-x64-musl, linux-arm64,
# linux-arm64-musl), reads `dist.integrity` and `dist.tarball` of
# @opencode/cli-<target>@<version> from the registry, downloads the .tgz,
# recomputes its sha512 (openssl dgst -sha512 -binary | base64) and fails if it
# differs from the announced integrity. On success it prints the fragment to
# add to "versions" in cli/internal/adapters/opencodev2/pinned_integrity.json.
#
# With --darwin, also prints the SHA-256 of the two macOS packages (the
# OPENCODE_SHA256_DARWIN_* values of .github/workflows/ci.yml), after the same
# check against their announced integrity.
#
# Run it twice, on two networks if possible, and compare the output: a value
# is only worth pinning if it is the same each time. The procedure is in
# cli/internal/adapters/opencodev2/pins.go.
#
# Needs curl, jq, openssl and shasum. OPENCODE_REGISTRY overrides the registry
# (default https://registry.npmjs.org).
set -euo pipefail

registry="${OPENCODE_REGISTRY:-https://registry.npmjs.org}"
registry="${registry%/}"
linux_targets=(linux-x64 linux-x64-musl linux-arm64 linux-arm64-musl)
darwin_targets=(darwin-arm64 darwin-x64)

usage() {
  echo "usage: $(basename "$0") [--darwin] <version>" >&2
  exit 2
}

darwin=0
version=""
for arg in "$@"; do
  case "$arg" in
    --darwin) darwin=1 ;;
    -h | --help) usage ;;
    -*) usage ;;
    *) [ -z "$version" ] || usage; version="$arg" ;;
  esac
done
[ -n "$version" ] || usage
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: '$version' is not a version (expected major.minor.patch)" >&2
  exit 2
fi

for tool in curl jq openssl shasum; do
  command -v "$tool" >/dev/null || { echo "error: $tool is required" >&2; exit 2; }
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# fetch <target> downloads @opencode/cli-<target>@<version>, checks the .tgz
# against the integrity the registry announces and sets $integrity and $archive.
fetch() {
  local target="$1" meta tarball recomputed
  meta="$work/$target.json"
  curl --fail --silent --show-error --location --retry 3 \
    --output "$meta" "$registry/@opencode%2Fcli-$target/$version"
  integrity="$(jq -er '.dist.integrity' "$meta")"
  tarball="$(jq -er '.dist.tarball' "$meta")"
  archive="$work/$target.tgz"
  echo "$target: $tarball" >&2
  curl --fail --silent --show-error --location --retry 3 --output "$archive" "$tarball"
  recomputed="sha512-$(openssl dgst -sha512 -binary "$archive" | base64 | tr -d '\n')"
  if [ "$recomputed" != "$integrity" ]; then
    echo "error: $target: the tarball does not match the announced integrity" >&2
    echo "  announced:  $integrity" >&2
    echo "  recomputed: $recomputed" >&2
    exit 1
  fi
}

declare -a linux_lines=()
for target in "${linux_targets[@]}"; do
  fetch "$target"
  linux_lines+=("$target|$integrity")
  rm -f "$archive"
done

declare -a darwin_lines=()
if [ "$darwin" -eq 1 ]; then
  for target in "${darwin_targets[@]}"; do
    fetch "$target"
    darwin_lines+=("$target|$(shasum -a 256 "$archive" | awk '{print $1}')")
    rm -f "$archive"
  done
fi

echo >&2
echo "All packages match the integrity announced by the registry." >&2
echo "To add under \"versions\" in cli/internal/adapters/opencodev2/pinned_integrity.json:" >&2
echo >&2
printf '    "%s": {\n' "$version"
last=$((${#linux_lines[@]} - 1))
for i in "${!linux_lines[@]}"; do
  sep=","
  [ "$i" -eq "$last" ] && sep=""
  printf '      "%s": "%s"%s\n' "${linux_lines[$i]%%|*}" "${linux_lines[$i]#*|}" "$sep"
done
printf '    }\n'

if [ "$darwin" -eq 1 ]; then
  echo >&2
  echo "SHA-256 of the macOS packages (.github/workflows/ci.yml):" >&2
  for line in "${darwin_lines[@]}"; do
    name="OPENCODE_SHA256_$(echo "${line%%|*}" | tr 'a-z-' 'A-Z_')"
    printf '%s: "%s"\n' "$name" "${line#*|}"
  done
fi
