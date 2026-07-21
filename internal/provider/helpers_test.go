// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// Environment variables used to point acceptance tests at real, account-owned
// DNS so the Urllo API accepts the created resources.
const (
	// envTestDomain is a domain the account controls (e.g. "unleashthe.cloud").
	// Rule tests create redirects on unique subdomains of it.
	envTestDomain = "URLLO_TEST_DOMAIN"
	// envTestHost is an existing host with valid DNS (e.g.
	// "urllo.unleashthe.cloud"), used by the host resource test.
	envTestHost = "URLLO_TEST_HOST"
)

// randInt returns a random integer for building unique test fixtures.
func randInt() int {
	return acctest.RandInt()
}

// testAccDomain returns the account-owned domain for rule tests, skipping the
// test when it is not configured.
func testAccDomain(t *testing.T) string {
	t.Helper()
	domain := os.Getenv(envTestDomain)
	if domain == "" {
		t.Skipf("%s must be set to a domain your Urllo account controls (e.g. unleashthe.cloud)", envTestDomain)
	}
	return domain
}

// testAccSource returns a unique source hostname under the account-owned domain.
func testAccSource(t *testing.T, prefix string) string {
	return fmt.Sprintf("%s-%d.%s", prefix, randInt(), testAccDomain(t))
}
