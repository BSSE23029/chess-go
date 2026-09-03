#!/usr/bin/env bash
# Reports Go files past the size limits (see SKILL.md).
#
#   check.sh                       every file under lib/ and test/
#   check.sh lib/a.rs lib/b.rs     just these
#   check.sh --hard                only report files over the hard ceiling
#
# 300 is the soft target: over it is a nudge, reported as OVER, exit 0.
# 500 is the hard ceiling: over it fails, reported as HARD, exit 1.
set -uo pipefail

SOFT=300
HARD=500
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

hard_only=0
files=()
for a in "$@"; do
  case "$a" in
    --hard) hard_only=1 ;;
    *) files+=("$a") ;;
  esac
done

exempt() {
  case "$1" in
    */target/*) return 0 ;;
    *) return 1 ;;
  esac
}

if [ "${#files[@]}" -eq 0 ]; then
  # NUL-delimited so a path with a space cannot split into two.
  while IFS= read -r -d '' f; do files+=("$f"); done \
    < <(find "$ROOT" -name target -prune -o -name '*.rs' -type f -print0 2>/dev/null)
fi

status=0
for f in "${files[@]}"; do
  case "$f" in *.rs) ;; *) continue ;; esac
  [ -f "$f" ] || continue
  exempt "$f" && continue
  n=$(wc -l < "$f" | tr -d ' ')
  rel="${f#"$ROOT"/}"
  if [ "$n" -gt "$HARD" ]; then
    printf 'HARD %s: %s lines (over the %s-line ceiling by %s) — must be split\n' \
      "$rel" "$n" "$HARD" "$((n - HARD))"
    status=1
  elif [ "$n" -gt "$SOFT" ] && [ "$hard_only" -eq 0 ]; then
    printf 'OVER %s: %s lines (over the %s-line target by %s) — split if there is a clean seam\n' \
      "$rel" "$n" "$SOFT" "$((n - SOFT))"
  fi
done

exit "$status"
