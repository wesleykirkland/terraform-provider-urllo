output "host_names" {
  description = "Every source host on the account."
  value       = [for h in data.urllo_hosts.all.hosts : h.name]
}

output "rule_count" {
  description = "Number of redirect rules on the account."
  value       = length(data.urllo_rules.all.rules)
}
