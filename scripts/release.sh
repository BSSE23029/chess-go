#!/bin/sh
set -eu

version=${VERSION:-}
dist=${DIST:-dist}
go_bin=${GO:-go}
cache=${GOCACHE:-/tmp/chess-go-build-cache}
if [ -z "$version" ] || [ "$version" = "dev" ]; then
	echo "release requires VERSION, for example VERSION=v0.2.0 make release-all" >&2
	exit 1
fi
case "$version" in
	*[!A-Za-z0-9._-]*)
		echo "release VERSION must contain only letters, digits, '.', '_' or '-'" >&2
		exit 1
		;;
esac

export GOCACHE="$cache"

targets=${RELEASE_TARGETS:-"darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"}
temporary=$(mktemp -d "${TMPDIR:-/tmp}/chess-go-release.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
mkdir -p "$dist/releases/$version"
release_dir=$(CDPATH= cd -- "$dist/releases/$version" && pwd)

for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	if [ -z "$os" ] || [ -z "$arch" ] || [ "$os" = "$target" ]; then
		echo "invalid release target: $target" >&2
		exit 1
	fi
	name="chess-go_${version}_${os}_${arch}"
	package_dir="$temporary/$name"
	mkdir -p "$package_dir"
	binary="$package_dir/chess"
	if [ "$os" = "windows" ]; then
		binary="$package_dir/chess.exe"
	fi
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$go_bin" build -trimpath -buildvcs=false \
		-ldflags "-s -w -buildid= -X main.version=$version" -o "$binary" ./cmd/chess
	if [ "$os" = "windows" ]; then
		( cd "$temporary" && zip -q -X -r "$release_dir/$name.zip" "$name" )
	else
		( cd "$temporary" && tar -czf "$release_dir/$name.tar.gz" "$name" )
	fi
done

checksum_file="$release_dir/SHA256SUMS"
: > "$checksum_file"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$release_dir"/*.tar.gz "$release_dir"/*.zip 2>/dev/null | sed "s#  $release_dir/#  #" > "$checksum_file"
else
	for artifact in "$release_dir"/*.tar.gz "$release_dir"/*.zip; do
		[ -f "$artifact" ] || continue
		shasum -a 256 "$artifact" | sed "s#  $release_dir/#  #" >> "$checksum_file"
	done
fi
echo "release artifacts: $release_dir"
