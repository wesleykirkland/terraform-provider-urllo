#!/usr/bin/env bash
#
# Builds the provider from source and installs it into a Terraform filesystem
# mirror so that `terraform init` can find it WITHOUT contacting a registry.
# Suitable for local development and CI.
#
# Usage:
#   ./terraform/build-local.sh [VERSION]
#
# Environment:
#   TF_PLUGIN_MIRROR  Target mirror directory
#                     (default: ~/.terraform.d/plugins, Terraform's implied
#                     local mirror, which `terraform init` searches with no extra
#                     CLI config). Set this to a repo-local path in CI, e.g.
#                     TF_PLUGIN_MIRROR="$PWD/.terraform-mirror".
set -euo pipefail

VERSION="${1:-0.0.1}"
NAMESPACE="wesleykirkland"
TYPE="urllo"
HOSTNAME="registry.terraform.io"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS="$(go env GOOS)"
ARCH="$(go env GOARCH)"
MIRROR="${TF_PLUGIN_MIRROR:-$HOME/.terraform.d/plugins}"

DEST="$MIRROR/$HOSTNAME/$NAMESPACE/$TYPE/$VERSION/${OS}_${ARCH}"
BINARY="terraform-provider-${TYPE}_v${VERSION}"

mkdir -p "$DEST"
( cd "$ROOT" && go build -ldflags "-s -w -X main.version=${VERSION}" -o "$DEST/$BINARY" . )

echo "installed terraform-provider-${TYPE} v${VERSION} (${OS}_${ARCH})"
echo "  -> $DEST/$BINARY"
echo
echo "If TF_PLUGIN_MIRROR is the default (~/.terraform.d/plugins), 'terraform init'"
echo "will find it automatically. Otherwise point Terraform at the mirror with a"
echo "CLI config (see mirror.tfrc.example) or TF_CLI_CONFIG_FILE."
