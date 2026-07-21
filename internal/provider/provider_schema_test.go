// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestProviderSchemaValid asks the provider server for its full schema, which
// exercises the provider, every resource, and every data-source schema. Any
// schema misconfiguration (e.g. an invalid Optional/Computed combination)
// surfaces as an error diagnostic here. Runs without credentials.
func TestProviderSchemaValid(t *testing.T) {
	factory := testAccProtoV6ProviderFactories["urllo"]
	server, err := factory()
	if err != nil {
		t.Fatalf("creating provider server: %v", err)
	}

	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s: %s", d.Summary, d.Detail)
		}
	}

	wantResources := []string{"urllo_rule", "urllo_host"}
	for _, name := range wantResources {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema %q", name)
		}
	}
	wantDataSources := []string{"urllo_rule", "urllo_rules", "urllo_host", "urllo_hosts"}
	for _, name := range wantDataSources {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("missing data source schema %q", name)
		}
	}
}
