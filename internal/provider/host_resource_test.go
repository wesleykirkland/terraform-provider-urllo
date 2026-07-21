// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccHostResource adopts an existing host and toggles its settings. Because
// hosts are provisioned via DNS (not the API), the test needs a real host to
// adopt (URLLO_TEST_HOST) and is skipped when that variable is unset.
func TestAccHostResource(t *testing.T) {
	host := os.Getenv(envTestHost)
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if host == "" {
				t.Skipf("%s must be set to an existing Urllo host for this test", envTestHost)
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostConfig(host, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("urllo_host.test", "name", host),
					resource.TestCheckResourceAttrSet("urllo_host.test", "id"),
					resource.TestCheckResourceAttr("urllo_host.test", "security.https_upgrade", "true"),
				),
			},
			{
				Config: testAccHostConfig(host, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("urllo_host.test", "security.https_upgrade", "false"),
				),
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

func testAccHostConfig(host string, httpsUpgrade bool) string {
	return fmt.Sprintf(`
resource "urllo_host" "test" {
  name = %q
  security = {
    https_upgrade = %t
  }
}
`, host, httpsUpgrade)
}
