// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// Attribute-type maps for the nested objects used by the host resource and data
// sources. They are the single source of truth for object shapes.
var (
	matchOptionsAttrTypes = map[string]attr.Type{
		"case_insensitive":  types.BoolType,
		"slash_insensitive": types.BoolType,
	}

	notFoundActionAttrTypes = map[string]attr.Type{
		"forward_params":          types.BoolType,
		"forward_path":            types.BoolType,
		"response_code":           types.Int64Type,
		"response_url":            types.StringType,
		"custom_404_body_present": types.BoolType,
	}

	securityAttrTypes = map[string]attr.Type{
		"https_upgrade":             types.BoolType,
		"prevent_foreign_embedding": types.BoolType,
		"hsts_include_sub_domains":  types.BoolType,
		"hsts_max_age":              types.Int64Type,
		"hsts_preload":              types.BoolType,
	}

	dnsValueAttrTypes = map[string]attr.Type{
		"type":   types.StringType,
		"values": types.ListType{ElemType: types.StringType},
	}

	dnsVerificationAttrTypes = map[string]attr.Type{
		"type":   types.StringType,
		"record": types.StringType,
		"value":  types.StringType,
	}
)

func requiredDNSAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"recommended":  types.ObjectType{AttrTypes: dnsValueAttrTypes},
		"alternatives": types.ListType{ElemType: types.ObjectType{AttrTypes: dnsValueAttrTypes}},
		"verification": types.ObjectType{AttrTypes: dnsVerificationAttrTypes},
	}
}

// --- API -> types.Object conversions (for reads / computed fields) -----------

func matchOptionsToObject(mo *client.MatchOptions, diags *diag.Diagnostics) types.Object {
	if mo == nil {
		return types.ObjectNull(matchOptionsAttrTypes)
	}
	obj, d := types.ObjectValue(matchOptionsAttrTypes, map[string]attr.Value{
		"case_insensitive":  types.BoolValue(mo.CaseInsensitive),
		"slash_insensitive": types.BoolValue(mo.SlashInsensitive),
	})
	diags.Append(d...)
	return obj
}

func notFoundActionToObject(nfa *client.NotFoundAction, diags *diag.Diagnostics) types.Object {
	if nfa == nil {
		return types.ObjectNull(notFoundActionAttrTypes)
	}
	obj, d := types.ObjectValue(notFoundActionAttrTypes, map[string]attr.Value{
		"forward_params":          types.BoolValue(nfa.ForwardParams),
		"forward_path":            types.BoolValue(nfa.ForwardPath),
		"response_code":           int64PtrValue(nfa.ResponseCode),
		"response_url":            stringPtrValue(nfa.ResponseURL),
		"custom_404_body_present": types.BoolValue(nfa.Custom404BodyPresent),
	})
	diags.Append(d...)
	return obj
}

func securityToObject(s *client.Security, diags *diag.Diagnostics) types.Object {
	if s == nil {
		return types.ObjectNull(securityAttrTypes)
	}
	obj, d := types.ObjectValue(securityAttrTypes, map[string]attr.Value{
		"https_upgrade":             types.BoolValue(s.HTTPSUpgrade),
		"prevent_foreign_embedding": types.BoolValue(s.PreventForeignEmbedding),
		"hsts_include_sub_domains":  types.BoolValue(s.HSTSIncludeSubDomains),
		"hsts_max_age":              int64PtrValue(s.HSTSMaxAge),
		"hsts_preload":              types.BoolValue(s.HSTSPreload),
	})
	diags.Append(d...)
	return obj
}

func dnsValueToObject(v client.DNSValue, diags *diag.Diagnostics) types.Object {
	values, d := types.ListValueFrom(context.Background(), types.StringType, v.Values)
	diags.Append(d...)
	obj, d := types.ObjectValue(dnsValueAttrTypes, map[string]attr.Value{
		"type":   types.StringValue(v.Type),
		"values": values,
	})
	diags.Append(d...)
	return obj
}

func detectedDNSToList(entries []client.DNSValue, diags *diag.Diagnostics) types.List {
	elemType := types.ObjectType{AttrTypes: dnsValueAttrTypes}
	if entries == nil {
		return types.ListNull(elemType)
	}
	objs := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		objs = append(objs, dnsValueToObject(e, diags))
	}
	list, d := types.ListValue(elemType, objs)
	diags.Append(d...)
	return list
}

func requiredDNSToObject(r *client.RequiredDNSValues, diags *diag.Diagnostics) types.Object {
	attrTypes := requiredDNSAttrTypes()
	if r == nil {
		return types.ObjectNull(attrTypes)
	}

	recommended := types.ObjectNull(dnsValueAttrTypes)
	if r.Recommended != nil {
		recommended = dnsValueToObject(*r.Recommended, diags)
	}

	altElem := types.ObjectType{AttrTypes: dnsValueAttrTypes}
	alternatives := types.ListNull(altElem)
	if r.Alternatives != nil {
		vals := make([]attr.Value, 0, len(r.Alternatives))
		for _, a := range r.Alternatives {
			vals = append(vals, dnsValueToObject(a, diags))
		}
		l, d := types.ListValue(altElem, vals)
		diags.Append(d...)
		alternatives = l
	}

	verification := types.ObjectNull(dnsVerificationAttrTypes)
	if r.Verification != nil {
		v, d := types.ObjectValue(dnsVerificationAttrTypes, map[string]attr.Value{
			"type":   types.StringValue(r.Verification.Type),
			"record": types.StringValue(r.Verification.Record),
			"value":  types.StringValue(r.Verification.Value),
		})
		diags.Append(d...)
		verification = v
	}

	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"recommended":  recommended,
		"alternatives": alternatives,
		"verification": verification,
	})
	diags.Append(d...)
	return obj
}

// --- types.Object -> API conversions (for writes) ----------------------------

// objectToMatchOptions returns nil when the object is null/unknown (i.e. not
// managed), so the API keeps its current value.
func objectToMatchOptions(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.MatchOptions {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var m struct {
		CaseInsensitive  types.Bool `tfsdk:"case_insensitive"`
		SlashInsensitive types.Bool `tfsdk:"slash_insensitive"`
	}
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	return &client.MatchOptions{
		CaseInsensitive:  m.CaseInsensitive.ValueBool(),
		SlashInsensitive: m.SlashInsensitive.ValueBool(),
	}
}

func objectToNotFoundAction(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.NotFoundAction {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var m struct {
		ForwardParams        types.Bool   `tfsdk:"forward_params"`
		ForwardPath          types.Bool   `tfsdk:"forward_path"`
		ResponseCode         types.Int64  `tfsdk:"response_code"`
		ResponseURL          types.String `tfsdk:"response_url"`
		Custom404BodyPresent types.Bool   `tfsdk:"custom_404_body_present"`
	}
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	// Custom404BodyPresent is computed by the API and never written here; the
	// actual body comes from the sibling custom_404_body resource attribute
	// and is attached by the caller.
	return &client.NotFoundAction{
		ForwardParams: m.ForwardParams.ValueBool(),
		ForwardPath:   m.ForwardPath.ValueBool(),
		ResponseCode:  int64ToIntPtr(m.ResponseCode),
		ResponseURL:   stringToPtr(m.ResponseURL),
	}
}

func objectToSecurity(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.Security {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var m struct {
		HTTPSUpgrade            types.Bool  `tfsdk:"https_upgrade"`
		PreventForeignEmbedding types.Bool  `tfsdk:"prevent_foreign_embedding"`
		HSTSIncludeSubDomains   types.Bool  `tfsdk:"hsts_include_sub_domains"`
		HSTSMaxAge              types.Int64 `tfsdk:"hsts_max_age"`
		HSTSPreload             types.Bool  `tfsdk:"hsts_preload"`
	}
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	return &client.Security{
		HTTPSUpgrade:            m.HTTPSUpgrade.ValueBool(),
		PreventForeignEmbedding: m.PreventForeignEmbedding.ValueBool(),
		HSTSIncludeSubDomains:   m.HSTSIncludeSubDomains.ValueBool(),
		HSTSMaxAge:              int64ToIntPtr(m.HSTSMaxAge),
		HSTSPreload:             m.HSTSPreload.ValueBool(),
	}
}

// --- scalar pointer helpers --------------------------------------------------

func int64PtrValue(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

func stringPtrValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func int64ToIntPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

func stringToPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
