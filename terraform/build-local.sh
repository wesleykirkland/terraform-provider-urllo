#!/usr/bin/env bash
#
# Builds the provider from source, installs it into a Terraform filesystem
# mirror, and initializes ./terraform against that build - WITHOUT contacting a
# registry. After this script finishes you can run `terraform plan`/`apply`
# directly. Suitable for local development and CI.
#
# Usage:
#   ./terraform/build-local.sh [VERSION]
#
# Steps:
#   1. go build the provider into the mirror.
#   2. Write a CLI config pointing Terraform at that mirror (only needed when
#      TF_PLUGIN_MIRROR overrides the default implied mirror).
#   3. Drop any stale .terraform/.terraform.lock.hcl in ./terraform, since a
#      rebuilt binary's hash will not match a previous lock file entry. This
#      never touches terraform.tfstate.
#   4. terraform init (resolves the provider from the mirror).
#   5. terraform validate, as a smoke test that requires no credentials.
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
REGISTRY_HOST="registry.terraform.io"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TF_DIR="$ROOT/terraform"
OS="$(go env GOOS)"
ARCH="$(go env GOARCH)"
DEFAULT_MIRROR="$HOME/.terraform.d/plugins"
MIRROR="${TF_PLUGIN_MIRROR:-$DEFAULT_MIRROR}"

DEST="$MIRROR/$REGISTRY_HOST/$NAMESPACE/$TYPE/$VERSION/${OS}_${ARCH}"
BINARY="terraform-provider-${TYPE}_v${VERSION}"

echo "==> Building terraform-provider-${TYPE} v${VERSION} for ${OS}_${ARCH}"
mkdir -p "$DEST"
( cd "$ROOT" && go build -ldflags "-s -w -X main.version=${VERSION}" -o "$DEST/$BINARY" . )
echo "    installed -> $DEST/$BINARY"

# Point Terraform at the mirror. The default implied mirror
# (~/.terraform.d/plugins) needs no CLI config - `terraform init` searches it
# automatically. A custom TF_PLUGIN_MIRROR (e.g. in CI) needs an explicit
# filesystem_mirror config so init resolves urllo from there instead of the
# registry, while other providers still install normally.
if [[ "$MIRROR" != "$DEFAULT_MIRROR" ]]; then
  MIRROR_TFRC="$TF_DIR/mirror.tfrc"
  cat >"$MIRROR_TFRC" <<TFRC
provider_installation {
  filesystem_mirror {
    path    = "$MIRROR"
    include = ["${REGISTRY_HOST}/${NAMESPACE}/${TYPE}"]
  }
  direct {
    exclude = ["${REGISTRY_HOST}/${NAMESPACE}/${TYPE}"]
  }
}
TFRC
  export TF_CLI_CONFIG_FILE="$MIRROR_TFRC"
  echo "==> Wrote $MIRROR_TFRC"
fi

echo "==> Initializing Terraform in $TF_DIR"
# A rebuilt binary has a new content hash, which would conflict with a lock
# file recorded against the previous build. Drop the provider lock/cache so
# init always succeeds against this rebuild; terraform.tfstate is untouched.
rm -rf "$TF_DIR/.terraform" "$TF_DIR/.terraform.lock.hcl"
terraform -chdir="$TF_DIR" init -input=false

echo "==> Validating configuration (no credentials required)"
terraform -chdir="$TF_DIR" validate

cat <<EOF

Ready. terraform-provider-${TYPE} v${VERSION} is installed and $TF_DIR is initialized.

Next steps:
  export URLLO_API_KEY=...
  export URLLO_API_SECRET=...
EOF
if [[ -n "${TF_CLI_CONFIG_FILE:-}" ]]; then
  echo "  export TF_CLI_CONFIG_FILE=\"$TF_CLI_CONFIG_FILE\"   # only needed again if you re-run init"
fi
echo "  terraform -chdir=\"$TF_DIR\" plan"
