// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRuleResource exercises create, update, and import of urllo_rule.
// DNS validation is disabled because the randomized test subdomains have no
// live DNS records. Requires TF_ACC, Urllo credentials, and URLLO_TEST_DOMAIN.
func TestAccRuleResource(t *testing.T) {
	source := testAccSource(t, "tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRuleConfig(source, "https://dest.example.com", "moved_permanently"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("urllo_rule.test", "id"),
					resource.TestCheckResourceAttr("urllo_rule.test", "target_url", "https://dest.example.com"),
					resource.TestCheckResourceAttr("urllo_rule.test", "response_type", "moved_permanently"),
					resource.TestCheckResourceAttr("urllo_rule.test", "source_urls.#", "1"),
				),
			},
			{
				Config: testAccRuleConfig(source, "https://dest2.example.com", "found"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("urllo_rule.test", "target_url", "https://dest2.example.com"),
					resource.TestCheckResourceAttr("urllo_rule.test", "response_type", "found"),
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

func testAccRuleConfig(source, target, responseType string) string {
	return fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls   = [%q]
  target_url    = %q
  response_type = %q
  validate_dns  = false
}
`, source, target, responseType)
}
