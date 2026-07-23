// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// providerData bundles the shared client with provider-level behavior toggles
// that Configure needs to hand down to resources and data sources.
type providerData struct {
	client             *client.Client
	includeDNSTestedAt bool
}

// clientFromProviderData extracts the shared *client.Client from ProviderData,
// appending a diagnostic on type mismatch. providerData is nil during early
// framework calls, in which case (nil, false) is returned without error.
func clientFromProviderData(data any, diags *diag.Diagnostics) (*client.Client, bool) {
	pd, ok := providerDataFrom(data, diags)
	if !ok {
		return nil, false
	}
	return pd.client, true
}

// providerDataFrom extracts the full providerData bundle, appending a
// diagnostic on type mismatch. providerData is nil during early framework
// calls, in which case (nil, false) is returned without error.
func providerDataFrom(data any, diags *diag.Diagnostics) (*providerData, bool) {
	if data == nil {
		return nil, false
	}
	pd, ok := data.(*providerData)
	if !ok {
		diags.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *providerData, got: %T. Please report this issue to the provider developers.", data),
		)
		return nil, false
	}
	return pd, true
}

// setToStrings converts a types.Set of strings into a Go slice. A null or
// unknown set yields a nil slice.
func setToStrings(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

// stringsToSet converts a Go slice into a types.Set of strings.
func stringsToSet(ctx context.Context, values []string, diags *diag.Diagnostics) types.Set {
	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return set
}
