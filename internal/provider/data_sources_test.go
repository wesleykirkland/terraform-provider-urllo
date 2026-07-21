// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRulesDataSource lists rules and reads a single rule created inline.
func TestAccRulesDataSource(t *testing.T) {
	source := testAccSource(t, "tf-acc-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRuleDataSourceConfig(source),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The single data source reflects the created rule.
					resource.TestCheckResourceAttr("data.urllo_rule.by_id", "target_url", "https://dest.example.com"),
					resource.TestCheckResourceAttr("data.urllo_rule.by_id", "source_urls.#", "1"),
					// The list data source returns at least one rule.
					resource.TestCheckResourceAttrWith("data.urllo_rules.all", "rules.#", func(v string) error {
						if v == "0" {
							return fmt.Errorf("expected at least one rule in list")
						}
						return nil
					}),
				),
			},
		},
	})
}

// TestAccHostsDataSource reads the list of hosts.
func TestAccHostsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "urllo_hosts" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.urllo_hosts.all", "hosts.#"),
				),
			},
		},
	})
}

func testAccRuleDataSourceConfig(source string) string {
	return fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls  = [%q]
  target_url   = "https://dest.example.com"
  validate_dns = false
}

data "urllo_rule" "by_id" {
  id = urllo_rule.test.id
}

data "urllo_rules" "all" {
  source_query = %q
}
`, source, source)
}
