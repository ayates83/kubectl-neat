#!/usr/bin/env bash
# This script makes a platform specific krew package
# it assumes goreleaser had already run and created the archives and the checksums
# Arguments:
#   1. target os (`linux`/`darwin`/`windows`)
#   2. target arch (`amd64`/`arm64`)
#   3. plugin name (rename the plugin in tests to avoid conflicts with existing installation)
#   4. path to goreleaser dist directory
# The version comes from $VERSION, else the checked-out tag, else v0.0.0.
set -euo pipefail

os="$1"
arch="$2"
plugin="$3"
dir="$4"
version="${VERSION:-$(git describe --tags --exact-match 2>/dev/null || echo v0.0.0)}"

"$(dirname "$0")/hack/krew-manifest.sh" "$version" "$dir" "$plugin" |
  OS="$os" ARCH="$arch" yq -P '.spec.platforms |= map(select(.selector.matchLabels.os == strenv(OS) and .selector.matchLabels.arch == strenv(ARCH)))' \
    >"$dir/kubectl-${plugin}_${os}_${arch}.yaml"
