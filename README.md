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
  # endpoint              = "https://api.urllo.com/v1"  # or URLLO_ENDPOINT (default shown)
  # include_dns_tested_at = false                       # opt in to a volatile read-only field (see below)
}

resource "urllo_rule" "marketing" {
  source_urls = ["example.com", "www.example.com"]
  target_url  = "https://www.newsite.com"
}
```

### Configuration

`api_key`, `api_secret`, and `endpoint` can each be supplied in HCL or via an
environment variable; explicit HCL values take precedence over the
environment. `include_dns_tested_at` is HCL-only (no environment variable).

| Setting               | Argument                | Environment variable | Default                    |
| --------------------- | ----------------------- | -------------------- | -------------------------- |
| API key               | `api_key`               | `URLLO_API_KEY`      | —                          |
| API secret            | `api_secret`            | `URLLO_API_SECRET`   | —                          |
| API endpoint          | `endpoint`              | `URLLO_ENDPOINT`     | `https://api.urllo.com/v1` |
| Include DNS tested-at | `include_dns_tested_at` | —                    | `false`                    |

Authentication uses HTTP Basic auth (API key as username, API secret as password).
The client automatically retries rate-limited (`429`) and `5xx` responses with
backoff, and sends an `Idempotency-Key` on every write.

`include_dns_tested_at` controls whether `urllo_host`'s read-only
`dns_tested_at` attribute is populated. It defaults to `false` because Urllo
re-tests DNS on its own schedule, independent of anything Terraform manages —
surfacing the timestamp would make `dns_tested_at` show as changed on every
refresh even though nothing actionable changed. Set it to `true` if you
actually want that timestamp in state.

## Resources and Data Sources

| Name | Type | Purpose |
| --- | --- | --- |
| [`urllo_rule`](docs/resources/rule.md) | Resource | Create/manage a redirect rule (`source_urls` → `target_url`, forwarding, tags, DNS validation). |
| [`urllo_host`](docs/resources/host.md) | Resource | Adopt an existing host by `name` and manage its match options, not-found behavior, and HTTPS/HSTS security settings. |
| [`urllo_rule`](docs/data-sources/rule.md) | Data source | Look up a single rule by `id`. |
| [`urllo_rules`](docs/data-sources/rules.md) | Data source | List rules, optionally filtered by source/target URL or tags. |
| [`urllo_host`](docs/data-sources/host.md) | Data source | Look up a single host by `id` or `name`. |
| [`urllo_hosts`](docs/data-sources/hosts.md) | Data source | List all source hosts. |

Full, generated attribute reference for every resource and data source lives
under [`docs/`](docs/) (rebuild it with `make generate` after changing a
schema). A quick summary of the two resources:

### `urllo_rule`

```hcl
resource "urllo_rule" "example" {
  source_urls   = ["example.com", "www.example.com"]
  target_url    = "https://www.newsite.com"
  response_type = "moved_permanently" # or "found"

  forward_params = true
  forward_path   = true
  tags           = ["marketing", "migration"]

  validate_dns         = true # default; see "DNS validation for rules" below
  validate_dns_timeout = "5m" # default
}
```

Read-only after creation: `id`, `name` (Urllo-assigned), `certificate_status`,
`dns_status`.

### `urllo_host`

Hosts must already exist in Urllo (added via the dashboard + DNS); this
resource *adopts* one by `name` rather than creating it, and destroying the
resource only removes it from state — it does not delete the host.

```hcl
resource "urllo_host" "example" {
  name = "www.example.com"

  acme_enabled = true

  match_options = {
    case_insensitive  = true
    slash_insensitive = true
  }

  not_found_action = {
    response_code  = 302 # 301, 302, or 404
    response_url   = "https://www.example.com"
    forward_params = true
    forward_path   = true
  }

  security = {
    https_upgrade             = true
    prevent_foreign_embedding = true
    hsts_include_sub_domains  = true
    hsts_max_age              = 31536000
    hsts_preload              = true
  }

  # Only used when not_found_action.response_code = 404. Write-only: the API
  # never returns this content, so drift in the body text can't be detected —
  # only whether a body is present, via not_found_action.custom_404_body_present.
  # custom_404_body = "<html>...</html>"
}
```

Read-only: `id`, `certificate_status`, `dns_status`, `detected_dns_entries`,
`required_dns_entries`, and `dns_tested_at` (only populated when the
provider's `include_dns_tested_at = true`).

### DNS validation for rules

Like [`aws_acm_certificate_validation`](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/acm_certificate_validation), `urllo_rule` can wait until each source
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
