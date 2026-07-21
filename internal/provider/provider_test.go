// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories instantiates the provider for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"urllo": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies the credentials required for acceptance testing are
// present, skipping the test when they are not.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		return
	}
	if os.Getenv(envAPIKey) == "" || os.Getenv(envAPISecret) == "" {
		t.Skipf("%s and %s must be set for acceptance tests", envAPIKey, envAPISecret)
	}
}
