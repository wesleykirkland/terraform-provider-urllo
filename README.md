# Terraform Provider for Urllo

A [Terraform](https://www.terraform.io) provider for the [Urllo](https://urllo.com)
redirection service, built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

It covers the entire Urllo API:

| Family | Terraform |
| ------ | --------- |
| Rules  | `urllo_rule` resource, `urllo_rule` / `urllo_rules` data sources |
| Hosts  | `urllo_host` resource, `urllo_host` / `urllo_hosts` data sources |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (to build)

## Using the Provider

```hcl
terraform {
  required_providers {
    urllo = {
      source = "wesleykirkland/urllo"
    }
  }
}

provider "urllo" {
  api_key    = var.urllo_api_key    # or URLLO_API_KEY
  api_secret = var.urllo_api_secret # or URLLO_API_SECRET
  # endpoint = "https://api.urllo.com/v1"  # or URLLO_ENDPOINT (default shown)
}

resource "urllo_rule" "marketing" {
  source_urls = ["example.com", "www.example.com"]
  target_url  = "https://www.newsite.com"
}
```

### Configuration

Every provider setting can be supplied in HCL or via an environment variable.
Explicit HCL values take precedence over environment variables.

| Setting      | Argument     | Environment variable | Default                    |
| ------------ | ------------ | -------------------- | -------------------------- |
| API key      | `api_key`    | `URLLO_API_KEY`      | —                          |
| API secret   | `api_secret` | `URLLO_API_SECRET`   | —                          |
| API endpoint | `endpoint`   | `URLLO_ENDPOINT`     | `https://api.urllo.com/v1` |

Authentication uses HTTP Basic auth (API key as username, API secret as password).
The client automatically retries rate-limited (`429`) and `5xx` responses with
backoff, and sends an `Idempotency-Key` on every write.

### DNS validation for rules

Like `aws_acm_certificate_validation`, `urllo_rule` can wait until each source
host's DNS resolves to the values Urllo requires before completing. This is
enabled by default; disable it with `validate_dns = false` (for example, before
you have cut DNS over):

```hcl
resource "urllo_rule" "example" {
  source_urls          = ["example.com"]
  target_url           = "https://dest.com"
  validate_dns         = true    # default
  validate_dns_timeout = "5m"    # default
}
```

## Developing the Provider

Requires [Go](http://www.golang.org). To build the provider and run it locally
against a real account without publishing to a registry, see
[`terraform/`](terraform/) for a ready-to-run dev-override example.

```shell
go install                 # build & install the provider binary
make test                  # unit tests (no credentials required)
make lint                  # golangci-lint
make generate              # regenerate docs (requires terraform)
make testacc               # acceptance tests (see below)
```

### Acceptance tests

Acceptance tests run through the real Terraform plugin protocol and are gated
behind `TF_ACC`. There are two flavours:

- **Mock-backed** (`TestAccMock*`) run the full provider CRUD against an
  in-memory Urllo API. They need **no credentials** and never touch your
  account, so CI runs them on every push. Just:

  ```shell
  TF_ACC=1 go test ./internal/provider/ -run TestAccMock
  ```

- **Live** tests create real resources against a Urllo account and additionally
  require credentials and a domain your account controls:

  ```shell
  export TF_ACC=1
  export URLLO_API_KEY=...
  export URLLO_API_SECRET=...
  export URLLO_TEST_DOMAIN=unleashthe.cloud   # rules are created on subdomains of this
  export URLLO_TEST_HOST=urllo.unleashthe.cloud  # optional: an existing host to manage
  make testacc
  ```

Live tests skip themselves when their required variables are absent. Note that
`TF_ACC` only needs to be **non-empty** to enable acceptance tests — `TF_ACC=0`
still enables them; unset the variable to disable.
