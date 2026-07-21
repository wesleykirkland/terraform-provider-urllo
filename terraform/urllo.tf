resource "urllo_rule" "example" {
  source_urls  = ["go.unleashthe.cloud"]
  target_url   = "https://unleashthe.cloud"
  validate_dns = false
}


# resource "urllo_rule" "example" {
#   source_urls  = ["go.unleashthe.cloud"]
#   target_url   = "https://unleashthe.cloud"
#   validate_dns = false
# }
