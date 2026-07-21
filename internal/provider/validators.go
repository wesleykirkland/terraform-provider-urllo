// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// durationValidator ensures a string attribute parses as a Go time.Duration.
type durationValidator struct{}

func (durationValidator) Description(context.Context) string {
	return "value must be a valid Go duration string (e.g. \"5m\")"
}

func (v durationValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (durationValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := time.ParseDuration(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid duration",
			"Expected a Go duration string such as \"5m\": "+err.Error())
	}
}
