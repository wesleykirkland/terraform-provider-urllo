# List all rules, optionally filtered by source/target URL and tags.
data "urllo_rules" "example" {
  source_query       = "example.com"
  tags               = ["marketing"]
  tag_match_strategy = "any"
}

output "matching_rule_ids" {
  value = [for r in data.urllo_rules.example.rules : r.id]
}
