resource "urllo_rule" "example" {
  source_urls  = ["go.unleashthe.cloud/"]
  target_url   = "https://unleashthe.cloud"
  validate_dns = false
}

resource "urllo_rule" "example3" {
  source_urls          = ["go3.unleashthe.cloud/"]
  target_url           = "https://unleashthe.cloud"
  validate_dns         = true
  validate_dns_timeout = "1m"

  response_type = "moved_permanently"

  forward_params = true
  forward_path   = true

  tags = ["marketing", "migration"]
}
