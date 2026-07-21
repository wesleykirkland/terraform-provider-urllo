output "host_names" {
  description = "Every source host on the account."
  value       = [for h in data.urllo_hosts.all.hosts : h.name]
}

output "rule_count" {
  description = "Number of redirect rules on the account."
  value       = length(data.urllo_rules.all.rules)
}

output "urllo_unleashthe_cloud" {
  description = "Details for the urllo.unleashthe.cloud host only."
  value = {
    id                 = data.urllo_host.urllo_unleashthe_cloud.id
    name               = data.urllo_host.urllo_unleashthe_cloud.name
    dns_status         = data.urllo_host.urllo_unleashthe_cloud.dns_status
    certificate_status = data.urllo_host.urllo_unleashthe_cloud.certificate_status
  }
}
