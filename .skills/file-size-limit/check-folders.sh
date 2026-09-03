#!/usr/bin/env bash
set -euo pipefail

root="${1:-lib}"
limit=5
failed=0

while IFS= read -r -d '' directory; do
  count=$(find "$directory" -maxdepth 1 -type f -print0 | tr -cd '\0' | wc -c | tr -d ' ')
  if (( count > limit )); then
    printf 'HARD %s: %d files (limit %d)\n' "$directory" "$count" "$limit"
    failed=1
  fi
done < <(find "$root" -path '*/target' -prune -o -type d -print0)

exit "$failed"
