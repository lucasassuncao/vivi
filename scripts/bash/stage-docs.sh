#!/usr/bin/env bash
# Assembles the docs site into <out-dir>: <docs-dir>/pages as the root, plus the
# markdown, the recordings and the icons beside it, where docsify and the landing
# page read them. GitHub Pages publishes this same directory.
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: stage-docs.sh <docs-dir> <out-dir>" >&2
	exit 2
fi

docs=$1
out=$2

rm -rf -- "$out"
mkdir -p "$out"
cp -r "$docs/pages/." "$out/"
cp "$docs"/*.md "$out/"
cp -r "$docs/images" "$docs/icons" "$out/"
echo "Docs site staged in $out/"
