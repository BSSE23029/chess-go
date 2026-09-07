#!/bin/sh
set -eu

release_dir=${1:-}
require_signature=${REQUIRE_RELEASE_SIGNATURE:-0}

if [ -z "$release_dir" ] || [ ! -d "$release_dir" ]; then
	echo "usage: scripts/verify-release.sh dist/releases/<version>" >&2
	exit 2
fi

checksum_file="$release_dir/SHA256SUMS"
if [ ! -s "$checksum_file" ]; then
	echo "missing or empty checksum manifest: $checksum_file" >&2
	exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
	( cd "$release_dir" && sha256sum -c SHA256SUMS )
elif command -v shasum >/dev/null 2>&1; then
	while IFS='  ' read -r expected artifact; do
		[ -n "$expected" ] && [ -n "$artifact" ] || continue
		actual=$(shasum -a 256 "$release_dir/$artifact" | awk '{print $1}')
		[ "$actual" = "$expected" ] || {
			echo "checksum mismatch: $artifact" >&2
			exit 1
		}
	done < "$checksum_file"
else
	echo "neither sha256sum nor shasum is available" >&2
	exit 1
fi

signature_found=0
for signature in "$checksum_file".asc "$checksum_file".sig; do
	if [ ! -f "$signature" ]; then
		continue
	fi
	signature_found=1
	if ! command -v gpg >/dev/null 2>&1; then
		echo "signature present but gpg is unavailable: $signature" >&2
		exit 1
	fi
	gpg --batch --verify "$signature" "$checksum_file"
done

if [ "$require_signature" = "1" ] && [ "$signature_found" -ne 1 ]; then
	echo "release signature required but no .asc or .sig manifest signature exists" >&2
	exit 1
fi

archive_count=$(find "$release_dir" -maxdepth 1 \( -name '*.tar.gz' -o -name '*.zip' \) -type f | wc -l | tr -d ' ')
manifest_count=$(awk 'NF == 2 { count++ } END { print count + 0 }' "$checksum_file")
if [ "$archive_count" -eq 0 ] || [ "$archive_count" -ne "$manifest_count" ]; then
	echo "checksum manifest does not cover every release archive" >&2
	exit 1
fi

echo "verified $archive_count release archive(s) in $release_dir"
