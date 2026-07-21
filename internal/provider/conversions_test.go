// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

func noErr(t *testing.T, d diag.Diagnostics) {
	t.Helper()
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
}

func TestMatchOptionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics

	// Nil -> null object -> nil.
	if obj := matchOptionsToObject(nil, &d); !obj.IsNull() {
		t.Fatal("nil MatchOptions should produce a null object")
	}
	if objectToMatchOptions(ctx, types.ObjectNull(matchOptionsAttrTypes), &d) != nil {
		t.Fatal("null object should convert back to nil")
	}

	in := &client.MatchOptions{CaseInsensitive: true, SlashInsensitive: false}
	obj := matchOptionsToObject(in, &d)
	noErr(t, d)
	out := objectToMatchOptions(ctx, obj, &d)
	noErr(t, d)
	if out == nil || out.CaseInsensitive != true || out.SlashInsensitive != false {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestNotFoundActionRoundTrip(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	code := 302
	url := "https://x.com"
	in := &client.NotFoundAction{ForwardParams: true, ForwardPath: false, ResponseCode: &code, ResponseURL: &url}
	obj := notFoundActionToObject(in, &d)
	noErr(t, d)
	out := objectToNotFoundAction(ctx, obj, &d)
	noErr(t, d)
	if out == nil || !out.ForwardParams || out.ResponseCode == nil || *out.ResponseCode != 302 || *out.ResponseURL != url {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestSecurityRoundTrip(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	age := 31536000
	in := &client.Security{HTTPSUpgrade: true, HSTSIncludeSubDomains: true, HSTSMaxAge: &age, HSTSPreload: true}
	obj := securityToObject(in, &d)
	noErr(t, d)
	out := objectToSecurity(ctx, obj, &d)
	noErr(t, d)
	if out == nil || !out.HTTPSUpgrade || out.HSTSMaxAge == nil || *out.HSTSMaxAge != age {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestRequiredDNSToObject(t *testing.T) {
	var d diag.Diagnostics
	if obj := requiredDNSToObject(nil, &d); !obj.IsNull() {
		t.Fatal("nil required should be null object")
	}
	in := &client.RequiredDNSValues{
		Recommended:  &client.DNSValue{Type: "CNAME", Values: []string{"t.urllo.com"}},
		Alternatives: []client.DNSValue{{Type: "A", Values: []string{"1.2.3.4"}}},
		Verification: &client.DNSVerification{Type: "TXT", Record: "_c.x", Value: "code"},
	}
	obj := requiredDNSToObject(in, &d)
	noErr(t, d)
	attrs := obj.Attributes()
	if attrs["recommended"].IsNull() || attrs["alternatives"].IsNull() || attrs["verification"].IsNull() {
		t.Fatalf("expected populated nested objects: %v", attrs)
	}
}

func TestDetectedDNSToList(t *testing.T) {
	var d diag.Diagnostics
	if l := detectedDNSToList(nil, &d); !l.IsNull() {
		t.Fatal("nil entries should be a null list")
	}
	l := detectedDNSToList([]client.DNSValue{{Type: "A", Values: []string{"1.2.3.4"}}}, &d)
	noErr(t, d)
	if len(l.Elements()) != 1 {
		t.Fatalf("expected 1 element, got %d", len(l.Elements()))
	}
}

func TestScalarPointerHelpers(t *testing.T) {
	if !int64PtrValue(nil).IsNull() {
		t.Error("nil int should be null")
	}
	if v := int64PtrValue(ptrInt(5)); v.ValueInt64() != 5 {
		t.Errorf("int64PtrValue = %d", v.ValueInt64())
	}
	if !stringPtrValue(nil).IsNull() {
		t.Error("nil string should be null")
	}
	if v := stringPtrValue(ptrStr("hi")); v.ValueString() != "hi" {
		t.Errorf("stringPtrValue = %q", v.ValueString())
	}
	if int64ToIntPtr(types.Int64Null()) != nil {
		t.Error("null int64 should convert to nil")
	}
	if p := int64ToIntPtr(types.Int64Value(7)); p == nil || *p != 7 {
		t.Errorf("int64ToIntPtr = %v", p)
	}
	if stringToPtr(types.StringNull()) != nil {
		t.Error("null string should convert to nil")
	}
	if p := stringToPtr(types.StringValue("x")); p == nil || *p != "x" {
		t.Errorf("stringToPtr = %v", p)
	}
}

func TestRuleToObjectAndList(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	rules := []client.Rule{
		{ID: "r1", Attributes: client.RuleAttributes{TargetURL: "d.com", SourceURLs: []string{"a.com"}, Tags: []string{"t"}}},
		{ID: "r2", Attributes: client.RuleAttributes{TargetURL: "e.com", SourceURLs: []string{"b.com"}}},
	}
	list := rulesToList(ctx, rules, &d)
	noErr(t, d)
	if len(list.Elements()) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(list.Elements()))
	}
	obj := ruleToObject(ctx, rules[1], &d)
	noErr(t, d)
	if obj.Attributes()["tags"].IsNull() != true {
		t.Error("expected null tags when none present")
	}
}

func TestSetStringConversions(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	set := stringsToSet(ctx, []string{"a", "b"}, &d)
	noErr(t, d)
	got := setToStrings(ctx, set, &d)
	noErr(t, d)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %v", got)
	}
	if setToStrings(ctx, types.SetNull(types.StringType), &d) != nil {
		t.Error("null set should convert to nil")
	}
}

func ptrInt(i int) *int       { return &i }
func ptrStr(s string) *string { return &s }
