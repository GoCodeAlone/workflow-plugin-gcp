#!/usr/bin/env bash
# update-plugin-version.sh: Stage a release-time plugin.json into .release/
# with the tag-bound version + download URLs. Source plugin.json stays at the
# "0.0.0" sentinel committed to the repo (workflow#758 plugin-version
# discipline) — only the tarball-shipped copy gets the real version.
#
# Used by goreleaser before hooks at release time.
# Usage: update-plugin-version.sh <version>
# Example: update-plugin-version.sh 0.2.0
set -euo pipefail

VERSION="${1:?usage: $0 <version>}"

mkdir -p .release
cp plugin.json .release/plugin.json

# Update "version" field
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/" .release/plugin.json

# Update version in release download URLs  (e.g. download/v0.1.0/ → download/v0.2.0/)
sed -i.bak "s|download/v[0-9][0-9.]*[^/]*/|download/v${VERSION}/|g" .release/plugin.json

# Update version in archive filenames     (e.g. workflow-plugin-gcp_0.1.0_ → _0.2.0_)
sed -i.bak "s|workflow-plugin-gcp_[0-9][0-9.]*[^_]*_|workflow-plugin-gcp_${VERSION}_|g" .release/plugin.json

rm -f .release/plugin.json.bak
echo ".release/plugin.json staged at version ${VERSION}"
