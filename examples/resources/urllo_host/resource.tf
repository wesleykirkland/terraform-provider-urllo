# Hosts are provisioned by adding a domain in the Urllo dashboard and configuring
# DNS. This resource adopts an existing host by name and manages its settings.
# Destroying it removes the resource from state only; the host is not deleted.
resource "urllo_host" "example" {
  name = "www.example.com"

  acme_enabled = true

  match_options = {
    case_insensitive  = true
    slash_insensitive = true
  }

  not_found_action = {
    forward_params = true
    forward_path   = true
    response_code  = 302
    response_url   = "https://www.example.com"
  }

  security = {
    https_upgrade             = true
    prevent_foreign_embedding = true
    hsts_include_sub_domains  = true
    hsts_max_age              = 31536000
    hsts_preload              = true
  }
}
