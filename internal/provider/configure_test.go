// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestProviderConfigureUnknownValues covers the branches that reject unknown
// credential values at plan time.
func TestProviderConfigureUnknownValues(t *testing.T) {
	ctx := context.Background()
	p := &UrlloProvider{version: "test"}

	var sr provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &sr)
	sc := sr.Schema
	objType := sc.Type().TerraformType(ctx)

	raw := tftypes.NewValue(objType, map[string]tftypes.Value{
		"api_key":               tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"api_secret":            tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"endpoint":              tftypes.NewValue(tftypes.String, nil),
		"include_dns_tested_at": tftypes.NewValue(tftypes.Bool, nil),
	})

	resp := &provider.ConfigureResponse{}
	p.Configure(ctx, provider.ConfigureRequest{
		Config: tfsdk.Config{Schema: sc, Raw: raw},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected errors for unknown api_key/api_secret")
	}
}

// badRaw is a value whose type does not match any resource/data-source model, so
// (Plan|State|Config).Get fails — exercising the defensive guard that returns
// early when Terraform cannot deserialize the request.
func badRaw() tftypes.Value {
	return tftypes.NewValue(tftypes.Bool, true)
}

// TestResourceGetGuards drives each resource CRUD method with an unreadable
// plan/state so the "cannot read request" guard returns an error.
func TestResourceGetGuards(t *testing.T) {
	ctx := context.Background()

	for _, r := range []resource.Resource{NewRuleResource(), NewHostResource()} {
		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		badPlan := tfsdk.Plan{Schema: sr.Schema, Raw: badRaw()}
		badState := tfsdk.State{Schema: sr.Schema, Raw: badRaw()}

		createResp := &resource.CreateResponse{}
		r.Create(ctx, resource.CreateRequest{Plan: badPlan}, createResp)
		if !createResp.Diagnostics.HasError() {
			t.Errorf("%T Create should error on unreadable plan", r)
		}

		readResp := &resource.ReadResponse{}
		r.Read(ctx, resource.ReadRequest{State: badState}, readResp)
		if !readResp.Diagnostics.HasError() {
			t.Errorf("%T Read should error on unreadable state", r)
		}

		updateResp := &resource.UpdateResponse{}
		r.Update(ctx, resource.UpdateRequest{Plan: badPlan, State: badState}, updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Errorf("%T Update should error on unreadable plan", r)
		}

		// HostResource.Delete is a warning-only no-op that never reads state, so
		// only the rule resource has a delete guard to trip.
		if _, ok := r.(*RuleResource); ok {
			deleteResp := &resource.DeleteResponse{}
			r.Delete(ctx, resource.DeleteRequest{State: badState}, deleteResp)
			if !deleteResp.Diagnostics.HasError() {
				t.Errorf("%T Delete should error on unreadable state", r)
			}
		}
	}
}

// TestDataSourceGetGuards drives each data source Read with an unreadable config.
func TestDataSourceGetGuards(t *testing.T) {
	ctx := context.Background()

	for _, d := range []datasource.DataSource{
		NewRuleDataSource(), NewRulesDataSource(), NewHostDataSource(),
	} {
		// urllo_hosts takes no input, so its Read never calls Config.Get.
		var sr datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &sr)
		resp := &datasource.ReadResponse{}
		d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: badRaw()}}, resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("%T Read should error on unreadable config", d)
		}
	}
}
