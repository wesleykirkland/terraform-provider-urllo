resource "urllo_rule" "example" {
  source_urls   = ["example.com", "www.example.com"]
  target_url    = "https://www.newsite.com"
  response_type = "moved_permanently"

  forward_params = true
  forward_path   = true

  tags = ["marketing", "migration"]

  # After create/update, wait until each source host's DNS resolves to the
  # values Urllo requires (like aws_acm_certificate_validation). Set to false to
  # skip, e.g. before DNS has been cut over.
  validate_dns         = true
  validate_dns_timeout = "5m"
}
