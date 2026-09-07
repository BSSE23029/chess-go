#!/bin/sh
set -eu

version=${VERSION:-}
dist=${DIST:-dist}
if [ -z "$version" ] || [ "$version" = "dev" ]; then
	echo "release preflight requires VERSION, for example VERSION=v0.1.0 make release-preflight" >&2
	exit 2
fi

head=$(git rev-parse HEAD)
tag=$(git rev-list -n 1 "$version" 2>/dev/null || true)
if [ -z "$tag" ]; then
	echo "release tag does not exist locally: $version" >&2
	exit 1
fi
if [ "$tag" != "$head" ]; then
	echo "release tag $version points to $tag, but HEAD is $head" >&2
	echo "review the release commit, then create or retarget the tag before publishing" >&2
	exit 1
fi

if ! git diff --quiet -- . ':(exclude).DS_Store'; then
	echo "tracked release files have uncommitted changes" >&2
	git status --short >&2
	exit 1
fi

sh scripts/verify-release.sh "$dist/releases/$version"
echo "release preflight passed for $version at $head"
