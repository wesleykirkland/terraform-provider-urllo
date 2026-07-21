# Running the Urllo provider from a local build

This directory builds the provider from source and runs it with Terraform
**without publishing it to a registry**. There are two ways to do that; pick
based on whether you need `terraform init` to work.

| Approach | `terraform init` | Best for |
| --- | --- | --- |
| **Filesystem mirror** (below) | ✅ works | CI, anything that runs `init` |
| **Dev overrides** (further down) | ❌ skipped | fast local iteration |

> `terraform init` **always** contacts the registry and **ignores** dev
> overrides, so an unpublished provider fails `init` with "registry ... does not
> have a provider named ...". The filesystem mirror fixes that.

## Filesystem mirror (works with `terraform init`)

### 1. Build into a local mirror

```shell
./build-local.sh            # installs v0.0.1 into ~/.terraform.d/plugins
```

`~/.terraform.d/plugins` is Terraform's *implied local mirror*, which `init`
searches automatically — so no extra CLI config is needed. Rebuild with the same
command after code changes.

### 2. Init, validate, plan

```shell
export URLLO_API_KEY=...
export URLLO_API_SECRET=...

terraform init       # resolves urllo from the local mirror, no registry
terraform validate   # no credentials needed
terraform plan       # reads your hosts and rules
```

### CI usage

In CI, install into a repo-local mirror and point Terraform at it explicitly so
other providers still resolve from the registry:

```shell
export TF_PLUGIN_MIRROR="$PWD/.terraform-mirror"
./terraform/build-local.sh                 # builds for the runner's OS/arch
cp terraform/mirror.tfrc.example mirror.tfrc
# set path in mirror.tfrc to "$TF_PLUGIN_MIRROR"
export TF_CLI_CONFIG_FILE="$PWD/mirror.tfrc"
terraform -chdir=terraform init
terraform -chdir=terraform plan
```

The mirror only holds one platform's binary; if your CI matrix spans platforms,
run `build-local.sh` on each, or `terraform providers lock -platform=...` to add
checksums.

## Dev overrides (fast iteration, no `init`)

For a quick edit-build-run loop where you don't want to reinstall into a mirror:

```shell
go install .                                   # from the repo root
cd terraform
printf 'provider_installation {\n  dev_overrides {\n    "registry.terraform.io/wesleykirkland/urllo" = "%s/bin"\n  }\n  direct {}\n}\n' "$(go env GOPATH)" > dev.tfrc

# Do NOT run `terraform init` with a dev override in effect.
TF_CLI_CONFIG_FILE=./dev.tfrc terraform validate
TF_CLI_CONFIG_FILE=./dev.tfrc terraform plan
```

`dev.tfrc` and the `.terraform*` working files are gitignored. `dev.tfrc.example`
and `mirror.tfrc.example` are committed templates.

## What the example does

- `provider.tf` — configures the provider (credentials from `URLLO_API_KEY` /
  `URLLO_API_SECRET`).
- `data.tf` — reads `urllo_hosts` and `urllo_rules` (safe, read-only).
- `outputs.tf` — prints the host names and rule count.
- `urllo.tf` — a commented `urllo_rule` resource; uncomment to manage a real
  redirect, then `terraform apply` / `destroy`.
