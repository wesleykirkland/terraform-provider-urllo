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

# Serves a custom body instead of Urllo's default page when no redirect rule
# matches. custom_404_body only takes effect when not_found_action.response_code
# is 404. not_found_action and security are independent settings applicable to
# both host configurations on this page — they're split across these two
# examples for readability, not because either is tied to one configuration.
resource "urllo_host" "custom_404" {
  name = "status.example.com"

  not_found_action = {
    response_code = 404
  }

  custom_404_body = <<-HTML
    <!doctype html>
    <html>
      <head><title>Page not found</title></head>
      <body><h1>404 - Page not found</h1></body>
    </html>
  HTML
}
