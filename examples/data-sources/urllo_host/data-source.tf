# Look up a single host by name (or by id).
data "urllo_host" "example" {
  name = "www.example.com"
}

output "host_dns_status" {
  value = data.urllo_host.example.dns_status
}
