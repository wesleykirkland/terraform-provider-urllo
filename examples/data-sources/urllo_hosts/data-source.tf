data "urllo_hosts" "all" {}

output "host_names" {
  value = [for h in data.urllo_hosts.all.hosts : h.name]
}
