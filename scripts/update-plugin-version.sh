#!/usr/bin/env bash
# update-plugin-version.sh: Update all version references in plugin.json.
# Used by goreleaser before hooks at release time.
# Usage: update-plugin-version.sh <version>
# Example: update-plugin-version.sh 0.2.0
set -euo pipefail

VERSION="${1:?usage: $0 <version>}"

# Update "version" field
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/" plugin.json

# Update version in release download URLs  (e.g. download/v0.1.0/ → download/v0.2.0/)
sed -i.bak "s|download/v[0-9][0-9.]*[^/]*/|download/v${VERSION}/|g" plugin.json

# Update version in archive filenames     (e.g. workflow-plugin-gcp_0.1.0_ → _0.2.0_)
sed -i.bak "s|workflow-plugin-gcp_[0-9][0-9.]*[^_]*_|workflow-plugin-gcp_${VERSION}_|g" plugin.json

rm -f plugin.json.bak
echo "plugin.json updated to version ${VERSION}"
