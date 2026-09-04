#!/bin/sh
set -eu

status=0
for file in $(rg --files -g '*.go'); do
	case "$file" in
	*_test.go) continue ;;
	esac
	lines=$(wc -l < "$file" | awk '{print $1}')
	if [ "$lines" -gt 300 ]; then
		echo "file-size warning: $file ($lines lines; soft target 300)"
	fi
	if [ "$lines" -ge 500 ]; then
		echo "file-size failure: $file ($lines lines; hard limit 500)" >&2
		status=1
	fi
done
exit "$status"
