// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// configureMockControl points the provider at a controllable mock server and
// supplies dummy credentials.
func configureMockControl(t *testing.T) *mockUrllo {
	t.Helper()
	srv, m := newMockUrlloServerWithControl(t)
	t.Setenv("URLLO_ENDPOINT", srv.URL)
	t.Setenv("URLLO_API_KEY", "mock")
	t.Setenv("URLLO_API_SECRET", "mock")
	return m
}

func ruleConfig(target string) string {
	return fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls  = [%q]
  target_url   = %q
  validate_dns = false
}
`, mockHostName, target)
}

func TestAccMockRuleCreateError(t *testing.T) {
	m := configureMockControl(t)
	m.setFailWriteOnce(http.StatusUnprocessableEntity)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      ruleConfig("https://dest.example.com"),
			ExpectError: regexp.MustCompile("Error creating rule"),
		}},
	})
}

func TestAccMockRuleUpdateError(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: ruleConfig("https://dest.example.com")},
			{
				PreConfig:   func() { m.setFailWriteOnce(http.StatusUnprocessableEntity) },
				Config:      ruleConfig("https://dest2.example.com"),
				ExpectError: regexp.MustCompile("Error updating rule"),
			},
		},
	})
}

// TestAccMockRuleDisappears exercises the Read not-found path: the rule is
// deleted out of band, so refresh removes it from state.
func TestAccMockRuleDisappears(t *testing.T) {
	m := configureMockControl(t)
	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ruleConfig("https://dest.example.com"),
				Check: resource.TestCheckResourceAttrWith("urllo_rule.test", "id", func(v string) error {
					id = v
					return nil
				}),
			},
			{
				PreConfig:          func() { m.deleteRule(id) },
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccMockRuleReadError(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: ruleConfig("https://dest.example.com")},
			{
				// 403 is not retried by the client, so the one-shot failure sticks.
				PreConfig:   func() { m.setFailOnce(http.StatusForbidden) },
				Config:      ruleConfig("https://dest.example.com"),
				ExpectError: regexp.MustCompile("Error reading rule"),
			},
		},
	})
}

func TestAccMockHostLookupError(t *testing.T) {
	m := configureMockControl(t)
	m.setFailOnce(http.StatusForbidden) // fails the GET /hosts lookup during Create
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      fmt.Sprintf(`resource "urllo_host" "test" { name = %q }`, mockHostName),
			ExpectError: regexp.MustCompile("Error looking up host"),
		}},
	})
}

func TestAccMockHostCreatePatchError(t *testing.T) {
	m := configureMockControl(t)
	m.setFailWriteOnce(http.StatusUnprocessableEntity) // GET lookup ok, PATCH fails
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      fmt.Sprintf(`resource "urllo_host" "test" { name = %q }`, mockHostName),
			ExpectError: regexp.MustCompile("Error updating host"),
		}},
	})
}

func TestAccMockHostReadError(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "urllo_host" "test" { name = %q }`, mockHostName)},
			{
				PreConfig:   func() { m.setFailOnce(http.StatusForbidden) },
				Config:      fmt.Sprintf(`resource "urllo_host" "test" { name = %q }`, mockHostName),
				ExpectError: regexp.MustCompile("Error reading host"),
			},
		},
	})
}

// TestAccMockRuleSourceChangeValidates updates a rule's source_urls, which
// re-runs DNS validation (validate_dns is left at its default of true; the
// seeded host has no required DNS so validation passes).
func TestAccMockRuleSourceChangeValidates(t *testing.T) {
	configureMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls = [%q]
  target_url  = "https://dest.example.com"
}
`, mockHostName)},
			{Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls = [%q, "extra.%s"]
  target_url  = "https://dest.example.com"
}
`, mockHostName, mockHostName)},
		},
	})
}

// TestAccMockHostDisappears covers the host Read not-found path: a 404 on
// refresh removes the resource from state.
func TestAccMockHostDisappears(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "urllo_host" "test" { name = %q }`, mockHostName)},
			{
				PreConfig:          func() { m.setFailOnce(http.StatusNotFound) },
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAccMockRuleDeleteError covers the Delete error branch: destroying the rule
// fails when the API rejects the delete.
func TestAccMockRuleDeleteError(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: ruleConfig("https://dest.example.com")},
			{
				Destroy:     true,
				PreConfig:   func() { m.setFailWriteOnce(http.StatusUnprocessableEntity) },
				Config:      ruleConfig("https://dest.example.com"),
				ExpectError: regexp.MustCompile("Error deleting rule"),
			},
		},
	})
}

// TestAccMockRuleDeleteNotFound covers the delete idempotency branch: a 404 on
// delete is treated as success.
func TestAccMockRuleDeleteNotFound(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: ruleConfig("https://dest.example.com")},
			{
				Destroy:   true,
				PreConfig: func() { m.setFailWriteOnce(http.StatusNotFound) },
				Config:    ruleConfig("https://dest.example.com"),
			},
		},
	})
}

// TestAccMockRuleCreateDNSValidationError covers the DNS-validation failure path
// during Create: the source host requires DNS that a local lookup can never
// satisfy, so validation times out.
func TestAccMockRuleCreateDNSValidationError(t *testing.T) {
	configureMock(t)
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	t.Cleanup(func() { dnsPollInterval = orig })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls          = [%q]
  target_url           = "https://dest.example.com"
  validate_dns         = true
  validate_dns_timeout = "200ms"
}
`, mockDNSFailHost),
			ExpectError: regexp.MustCompile("DNS validation timed out"),
		}},
	})
}

// TestAccMockRuleUpdateDNSValidationError covers the DNS-validation failure path
// during Update, when source_urls change to a host with unsatisfiable DNS.
func TestAccMockRuleUpdateDNSValidationError(t *testing.T) {
	configureMock(t)
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	t.Cleanup(func() { dnsPollInterval = orig })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: ruleConfig("https://dest.example.com")},
			{
				Config: fmt.Sprintf(`
resource "urllo_rule" "test" {
  source_urls          = [%q]
  target_url           = "https://dest.example.com"
  validate_dns         = true
  validate_dns_timeout = "200ms"
}
`, mockDNSFailHost),
				ExpectError: regexp.MustCompile("DNS validation timed out"),
			},
		},
	})
}

func TestAccMockHostNotFound(t *testing.T) {
	configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      `resource "urllo_host" "test" { name = "does-not-exist.example" }`,
			ExpectError: regexp.MustCompile("Host not found"),
		}},
	})
}

func TestAccMockHostUpdateError(t *testing.T) {
	m := configureMockControl(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "urllo_host" "test" {
  name = %q
}`, mockHostName)},
			{
				PreConfig: func() { m.setFailWriteOnce(http.StatusUnprocessableEntity) },
				Config: fmt.Sprintf(`resource "urllo_host" "test" {
  name         = %q
  acme_enabled = true
}`, mockHostName),
				ExpectError: regexp.MustCompile("Error updating host"),
			},
		},
	})
}

// TestAccMockDataSourceErrors covers the Read error branch of every data source.
func TestAccMockDataSourceErrors(t *testing.T) {
	cases := map[string]string{
		"rule":  `data "urllo_rule" "x" { id = "r1" }`,
		"rules": `data "urllo_rules" "x" {}`,
		"host":  `data "urllo_host" "x" { name = "any.example" }`,
		"hosts": `data "urllo_hosts" "x" {}`,
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			m := configureMockControl(t)
			m.setFail(http.StatusForbidden)
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      cfg,
					ExpectError: regexp.MustCompile("(?i)error (reading|listing)"),
				}},
			})
		})
	}
}

// TestAccMockHostDataSourceSelectors covers the host data source's missing
// selector and not-found branches.
func TestAccMockHostDataSourceSelectors(t *testing.T) {
	configureMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "urllo_host" "x" {}`,
				ExpectError: regexp.MustCompile("Missing host selector"),
			},
			{
				Config:      `data "urllo_host" "x" { name = "nope.example" }`,
				ExpectError: regexp.MustCompile("Host not found"),
			},
		},
	})
}

// TestAccMockMissingCredentials covers the provider Configure missing-credential
// branch by clearing the environment and supplying an empty api_key.
func TestAccMockMissingCredentials(t *testing.T) {
	srv, _ := newMockUrlloServerWithControl(t)
	t.Setenv("URLLO_ENDPOINT", srv.URL)
	t.Setenv("URLLO_API_KEY", "")
	t.Setenv("URLLO_API_SECRET", "")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "urllo" {
  api_key    = ""
  api_secret = ""
}

data "urllo_hosts" "x" {}
`,
			ExpectError: regexp.MustCompile("Missing Urllo API key"),
		}},
	})
}
