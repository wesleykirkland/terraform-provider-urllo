data "urllo_hosts" "all" {}

data "urllo_rules" "all" {}

# Single-host lookup for one specific host.
data "urllo_host" "urllo_unleashthe_cloud" {
  name = "urllo.unleashthe.cloud"
}
