# Does not require an import as the host already exists in Urllo
resource "urllo_host" "example3" {
  name = trim(one(urllo_rule.example3.source_urls), "/") # Urllo normalizes the hostname with a trailing /

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
# is 404
resource "urllo_host" "custom_404" {
  name = "urllo.unleashthe.cloud"

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
