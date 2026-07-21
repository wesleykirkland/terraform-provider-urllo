// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// ruleObjectAttrTypes is the shape of a rule object in the urllo_rules list.
var ruleObjectAttrTypes = map[string]attr.Type{
	"id":             types.StringType,
	"source_urls":    types.SetType{ElemType: types.StringType},
	"target_url":     types.StringType,
	"response_type":  types.StringType,
	"forward_params": types.BoolType,
	"forward_path":   types.BoolType,
	"tags":           types.SetType{ElemType: types.StringType},
}

func ruleToObject(ctx context.Context, rule client.Rule, diags *diag.Diagnostics) types.Object {
	sources, d := types.SetValueFrom(ctx, types.StringType, rule.Attributes.SourceURLs)
	diags.Append(d...)

	tags := types.SetNull(types.StringType)
	if len(rule.Attributes.Tags) > 0 {
		t, d := types.SetValueFrom(ctx, types.StringType, rule.Attributes.Tags)
		diags.Append(d...)
		tags = t
	}

	obj, d := types.ObjectValue(ruleObjectAttrTypes, map[string]attr.Value{
		"id":             types.StringValue(rule.ID),
		"source_urls":    sources,
		"target_url":     types.StringValue(rule.Attributes.TargetURL),
		"response_type":  types.StringValue(rule.Attributes.ResponseType),
		"forward_params": types.BoolValue(rule.Attributes.ForwardParams),
		"forward_path":   types.BoolValue(rule.Attributes.ForwardPath),
		"tags":           tags,
	})
	diags.Append(d...)
	return obj
}

func rulesToList(ctx context.Context, rules []client.Rule, diags *diag.Diagnostics) types.List {
	elemType := types.ObjectType{AttrTypes: ruleObjectAttrTypes}
	objs := make([]attr.Value, 0, len(rules))
	for _, r := range rules {
		objs = append(objs, ruleToObject(ctx, r, diags))
	}
	list, d := types.ListValue(elemType, objs)
	diags.Append(d...)
	return list
}
