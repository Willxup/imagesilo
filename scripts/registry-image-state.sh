#!/usr/bin/env bash
set -euo pipefail

reference="${1:?image reference is required}"
if ! command -v docker >/dev/null 2>&1 || ! docker buildx version >/dev/null 2>&1; then
	printf 'Docker Buildx is required for fail-closed registry inspection\n' >&2
	exit 1
fi
output_file="$(mktemp)"
error_file="$(mktemp)"
cleanup() {
  rm -f "$output_file" "$error_file"
}
trap cleanup EXIT INT TERM

if docker buildx imagetools inspect "$reference" >"$output_file" 2>"$error_file"; then
  digest="$(awk '$1 == "Digest:" { print $2; exit }' "$output_file")"
	if [[ ! "$digest" =~ ^sha256:[0-9a-f]{64}$ ]]; then
	  printf 'registry returned an invalid digest for %s\n' "$reference" >&2
	  cat "$output_file" >&2
	  exit 1
	fi
  printf 'present %s\n' "$digest"
  exit 0
fi

if grep -Eqi 'manifest unknown|name unknown|not found|does not exist' "$error_file"; then
  printf 'absent\n'
  exit 0
fi

printf 'registry lookup failed for %s; refusing to treat the result as absent\n' "$reference" >&2
cat "$error_file" >&2
exit 1
