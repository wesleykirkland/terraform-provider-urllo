data "urllo_rule" "example" {
  id = "abc-def"
}

output "rule_target" {
  value = data.urllo_rule.example.target_url
}
