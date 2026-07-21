// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The TestAccMock* tests run the full provider CRUD through the real Terraform
// plugin protocol against an in-memory Urllo API (mock_server_test.go). They are
// gated on TF_ACC like other acceptance tests but need no real credentials, so
// CI can run them without secrets. They point the provider at the mock server
// and supply dummy credentials via the environment.

func configureMock(t *testing.T) {
	t.Helper()
	srv := newMockUrlloServer(t)
	t.Setenv("URLLO_ENDPOINT", srv.URL)
	t.Setenv("URLLO_API_KEY", "mock")
	t.Setenv("URLLO_API_SECRET", "mock")
}

func TestAccMockRuleResource(t *testing.T) {
	configureMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// validate_dns defaults to true. The seeded mock host has no
				// required DNS entries, so local validation passes immediately.
				Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls = [%q]
  target_url  = "https://dest.example.com"
  tags        = ["a", "b"]
}
`, mockHostName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("urllo_rule.test", "id"),
					resource.TestCheckResourceAttr("urllo_rule.test", "response_type", "moved_permanently"),
					resource.TestCheckResourceAttr("urllo_rule.test", "forward_params", "false"),
					resource.TestCheckResourceAttr("urllo_rule.test", "tags.#", "2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls    = [%q]
  target_url     = "https://dest2.example.com"
  response_type  = "found"
  forward_params = true
  validate_dns   = false
}
`, mockHostName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("urllo_rule.test", "target_url", "https://dest2.example.com"),
					resource.TestCheckResourceAttr("urllo_rule.test", "response_type", "found"),
					resource.TestCheckResourceAttr("urllo_rule.test", "forward_params", "true"),
				),
			},
			{
				ResourceName:      "urllo_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The Urllo API normalizes URLs server-side (e.g. adding a
				// trailing slash, adding a scheme to bare hostnames), so a
				// fresh import reflects the API's normalized form rather than
				// the originally-configured string, even though both refer to
				// the same redirect. See applyRuleToModel for the write side of
				// this trade-off.
				ImportStateVerifyIgnore: []string{"validate_dns", "validate_dns_timeout", "target_url", "source_urls"},
			},
		},
	})
}

func TestAccMockHostResource(t *testing.T) {
	configureMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "urllo_host" "test" {
  name         = %q
  acme_enabled = true
  security = {
    https_upgrade = true
    hsts_max_age  = 31536000
  }
  match_options = {
    case_insensitive = true
  }
}
`, mockHostName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("urllo_host.test", "id", "host-1"),
					resource.TestCheckResourceAttr("urllo_host.test", "name", mockHostName),
					resource.TestCheckResourceAttr("urllo_host.test", "acme_enabled", "true"),
					resource.TestCheckResourceAttr("urllo_host.test", "security.https_upgrade", "true"),
					resource.TestCheckResourceAttr("urllo_host.test", "dns_status", "active"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "urllo_host" "test" {
  name         = %q
  acme_enabled = false
  security = {
    https_upgrade = false
  }
}
`, mockHostName),
				Check: resource.TestCheckResourceAttr("urllo_host.test", "acme_enabled", "false"),
			},
			{
				ResourceName:            "urllo_host.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"custom_404_body"},
			},
		},
	})
}

func TestAccMockDataSources(t *testing.T) {
	configureMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls  = [%q]
  target_url   = "https://dest.example.com"
  validate_dns = false
}

data "urllo_rule" "by_id" {
  id = urllo_rule.test.id
}

# depends_on defers the list read until after the rule is created.
data "urllo_rules" "all" {
  depends_on = [urllo_rule.test]
}

data "urllo_host" "one" {
  name = %q
}

data "urllo_host" "byid" {
  id = "host-1"
}

data "urllo_hosts" "all" {}
`, mockHostName, mockHostName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The mock (like the real Urllo API) normalizes a path-less
					// target_url by appending a trailing slash; data sources
					// faithfully report the server's actual value.
					resource.TestCheckResourceAttr("data.urllo_rule.by_id", "target_url", "https://dest.example.com/"),
					resource.TestCheckResourceAttr("data.urllo_rules.all", "rules.#", "1"),
					resource.TestCheckResourceAttr("data.urllo_host.one", "id", "host-1"),
					resource.TestCheckResourceAttr("data.urllo_host.one", "dns_status", "active"),
					resource.TestCheckResourceAttr("data.urllo_host.byid", "name", mockHostName),
					// The mock seeds two hosts; list order is not guaranteed.
					resource.TestCheckResourceAttr("data.urllo_hosts.all", "hosts.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("data.urllo_hosts.all", "hosts.*", map[string]string{"name": mockHostName}),
				),
			},
		},
	})
}
