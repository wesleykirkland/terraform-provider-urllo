terraform {
  required_providers {
    urllo = {
      source = "wesleykirkland/urllo"
    }
  }
}

# Credentials are read from the URLLO_API_KEY / URLLO_API_SECRET environment
# variables. You can also set them explicitly here (not recommended for real
# secrets):
#
#   provider "urllo" {
#     api_key    = var.urllo_api_key
#     api_secret = var.urllo_api_secret
#   }
provider "urllo" {}

# ---------------------------------------------------------------------------
# Data sources (read-only) — safe to run against a real account.
# ---------------------------------------------------------------------------

data "urllo_hosts" "all" {}

data "urllo_rules" "all" {}

# ---------------------------------------------------------------------------
# Example managed rule. Uncomment to create a real redirect. Point source_urls
# at a domain your account controls. validate_dns waits until the source host's
# DNS resolves to Urllo's values before completing (set false to skip).
# ---------------------------------------------------------------------------

# resource "urllo_rule" "example" {
#   source_urls  = ["go.unleashthe.cloud"]
#   target_url   = "https://unleashthe.cloud"
#   validate_dns = false
# }

output "host_names" {
  description = "Every source host on the account."
  value       = [for h in data.urllo_hosts.all.hosts : h.name]
}

output "rule_count" {
  description = "Number of redirect rules on the account."
  value       = length(data.urllo_rules.all.rules)
}
