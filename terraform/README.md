# Running the Urllo provider locally

This directory is a self-contained example for building the provider from source
and running it with Terraform **without publishing it to a registry**, using
Terraform's [development overrides](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).

## 1. Build & install the provider

From the repository root:

```shell
go install .
```

This compiles the provider and places the `terraform-provider-urllo` binary in
your `GOPATH/bin` (equivalent to `make install`). Confirm the location:

```shell
go env GOPATH   # the binary is in <GOPATH>/bin
```

## 2. Point Terraform at the local build

Copy the example CLI config and set the path to your `GOPATH/bin`:

```shell
cd terraform
cp dev.tfrc.example dev.tfrc
# Edit dev.tfrc: replace /Users/CHANGE_ME/go/bin with "$(go env GOPATH)/bin".
```

Or generate it in one line (macOS/Linux):

```shell
printf 'provider_installation {\n  dev_overrides {\n    "registry.terraform.io/wesleykirkland/urllo" = "%s/bin"\n  }\n  direct {}\n}\n' "$(go env GOPATH)" > dev.tfrc
```

With a dev override in effect you **do not** (and must not) run
`terraform init` — Terraform uses the binary directly.

## 3. Provide credentials

```shell
export URLLO_API_KEY=...
export URLLO_API_SECRET=...
# Optional: point at a non-default API base URL.
# export URLLO_ENDPOINT=https://api.urllo.com/v1
```

## 4. Run it

```shell
# Validate the config against the live provider schema (no credentials/API needed):
TF_CLI_CONFIG_FILE=./dev.tfrc terraform validate

# Plan against your real account (reads hosts and rules):
TF_CLI_CONFIG_FILE=./dev.tfrc terraform plan
```

You should see your account's hosts in the `host_names` output and a
`rule_count`. Uncomment the `urllo_rule` resource in `main.tf` to create a real
redirect, then `terraform apply` / `terraform destroy`.

> Tip: instead of passing `TF_CLI_CONFIG_FILE` every time, you can add the same
> `dev_overrides` block to your user-wide `~/.terraformrc`.

## Rebuilding after code changes

Re-run `go install .` from the repo root. Terraform picks up the new binary on
the next command — no re-init required.
