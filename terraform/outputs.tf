output "host_names" {
  description = "Every source host on the account."
  value       = [for h in data.urllo_hosts.all.hosts : h.name]
}

output "rule_count" {
  description = "Number of redirect rules on the account."
  value       = length(data.urllo_rules.all.rules)
}

output "custom_404_body_present" {
  description = "Drift check for the custom_404 host: whether a custom 404 body is currently set, reflecting the resource's own refreshed state rather than a separate data source lookup."
  value       = urllo_host.custom_404.not_found_action.custom_404_body_present
}

output "urllo_rule_example3" {
  description = "Full output from example 3"
  value = urllo_rule.example3
}
