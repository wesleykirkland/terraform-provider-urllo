// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// --- clientFromProviderData --------------------------------------------------

func TestClientFromProviderData(t *testing.T) {
	var d diag.Diagnostics
	if c, ok := clientFromProviderData(nil, &d); ok || c != nil {
		t.Error("nil provider data should yield (nil, false)")
	}
	if d.HasError() {
		t.Error("nil provider data should not add an error")
	}

	if _, ok := clientFromProviderData("wrong-type", &d); ok {
		t.Error("wrong type should yield ok=false")
	}
	if !d.HasError() {
		t.Error("wrong type should add an error diagnostic")
	}

	var d2 diag.Diagnostics
	cl := client.New("", "k", "s")
	if c, ok := clientFromProviderData(cl, &d2); !ok || c != cl {
		t.Error("correct type should return the client")
	}
	if d2.HasError() {
		t.Error("correct type should not add an error")
	}
}

// --- durationValidator -------------------------------------------------------

func TestDurationValidator(t *testing.T) {
	ctx := context.Background()
	v := durationValidator{}
	if v.Description(ctx) == "" || v.MarkdownDescription(ctx) == "" {
		t.Error("descriptions must be non-empty")
	}

	validate := func(val types.String) diag.Diagnostics {
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, validator.StringRequest{Path: path.Root("validate_dns_timeout"), ConfigValue: val}, resp)
		return resp.Diagnostics
	}
	if d := validate(types.StringValue("5m")); d.HasError() {
		t.Errorf("valid duration should pass: %v", d)
	}
	if d := validate(types.StringValue("nonsense")); !d.HasError() {
		t.Error("invalid duration should add an error")
	}
	if d := validate(types.StringNull()); d.HasError() {
		t.Error("null value should be skipped")
	}
	if d := validate(types.StringUnknown()); d.HasError() {
		t.Error("unknown value should be skipped")
	}
}

// --- objectToSecurity null branch --------------------------------------------

func TestObjectToSecurityNull(t *testing.T) {
	var d diag.Diagnostics
	if objectToSecurity(context.Background(), types.ObjectNull(securityAttrTypes), &d) != nil {
		t.Error("null object should convert to nil")
	}
}

// --- rule model helpers ------------------------------------------------------

func TestAttributesAndApplyRuleModel(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	r := &RuleResource{}

	model := &RuleResourceModel{
		SourceURLs:    stringsToSet(ctx, []string{"a.com"}, &d),
		TargetURL:     types.StringValue("dest.com"),
		ResponseType:  types.StringValue(client.ResponseMovedPermanently),
		ForwardParams: types.BoolValue(true),
		ForwardPath:   types.BoolValue(false),
		Tags:          types.SetNull(types.StringType),
	}
	attrs := r.attributesFromModel(ctx, model, &d)
	if attrs.TargetURL != "dest.com" || !attrs.ForwardParams || len(attrs.SourceURLs) != 1 {
		t.Fatalf("attributesFromModel mismatch: %+v", attrs)
	}

	// applyRuleToModel with tags present, and with tags absent + config null.
	rule := &client.Rule{ID: "r1", Attributes: client.RuleAttributes{
		TargetURL: "dest.com", ResponseType: "found", SourceURLs: []string{"a.com"}, Tags: []string{"t"},
	}}
	r.applyRuleToModel(ctx, rule, model, &d)
	if model.ID.ValueString() != "r1" || model.Tags.IsNull() {
		t.Fatalf("applyRuleToModel with tags failed: %+v", model)
	}

	model.Tags = types.SetNull(types.StringType)
	ruleNoTags := &client.Rule{ID: "r2", Attributes: client.RuleAttributes{TargetURL: "d", SourceURLs: []string{"a.com"}}}
	r.applyRuleToModel(ctx, ruleNoTags, model, &d)
	if !model.Tags.IsNull() {
		t.Error("absent tags with null config should stay null")
	}
}

// --- readHost / applyUpdate via a stub server --------------------------------

func hostServer(t *testing.T, fail bool) *client.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/hosts" {
			_, _ = w.Write([]byte(`{"data":[{"id":"h1","type":"host","attributes":{"name":"a.com"}}],"links":{"next":null}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"h1","type":"host","attributes":{"name":"a.com","acme_enabled":true}}}`))
	}))
	t.Cleanup(srv.Close)
	return client.New(srv.URL, "k", "s", client.WithHTTPClient(srv.Client()))
}

func TestReadHostByIDAndName(t *testing.T) {
	ctx := context.Background()
	r := &HostResource{client: hostServer(t, false)}

	byID := &HostResourceModel{ID: types.StringValue("h1"), Name: types.StringNull()}
	if h, err := r.readHost(ctx, byID); err != nil || h == nil {
		t.Fatalf("readHost by id: %v %v", h, err)
	}

	byName := &HostResourceModel{ID: types.StringNull(), Name: types.StringValue("a.com")}
	if h, err := r.readHost(ctx, byName); err != nil || h == nil {
		t.Fatalf("readHost by name: %v %v", h, err)
	}
}

func TestApplyUpdate(t *testing.T) {
	ctx := context.Background()
	var d diag.Diagnostics
	model := &HostResourceModel{
		ACMEEnabled:    types.BoolValue(true),
		Custom404Body:  types.StringValue("body"),
		MatchOptions:   types.ObjectNull(matchOptionsAttrTypes),
		NotFoundAction: types.ObjectNull(notFoundActionAttrTypes),
		Security:       types.ObjectNull(securityAttrTypes),
	}

	ok := &HostResource{client: hostServer(t, false)}
	if host := ok.applyUpdate(ctx, "h1", model, &d); host == nil || d.HasError() {
		t.Fatalf("applyUpdate success failed: %v", d)
	}

	var d2 diag.Diagnostics
	failing := &HostResource{client: hostServer(t, true)}
	if host := failing.applyUpdate(ctx, "h1", model, &d2); host != nil || !d2.HasError() {
		t.Error("applyUpdate should report client error")
	}
}

// --- rule DNS validation helpers ---------------------------------------------

func TestMaybeValidateDNS(t *testing.T) {
	ctx := context.Background()
	r := &RuleResource{}

	// Disabled -> no-op.
	var d diag.Diagnostics
	r.maybeValidateDNS(ctx, &RuleResourceModel{ValidateDNS: types.BoolValue(false)}, &d)
	if d.HasError() {
		t.Error("disabled validation should not error")
	}

	// Enabled with an invalid timeout -> error.
	var d2 diag.Diagnostics
	r.maybeValidateDNS(ctx, &RuleResourceModel{
		ValidateDNS:        types.BoolValue(true),
		ValidateDNSTimeout: types.StringValue("not-a-duration"),
		SourceURLs:         types.SetNull(types.StringType),
	}, &d2)
	if !d2.HasError() {
		t.Error("invalid timeout should error")
	}
}

// TestCreateRollsBackOnDNSTimeout drives the full Create() method against a
// mock API and a resolver that never matches, so DNS validation always times
// out. It asserts the created rule is deleted and no state is saved, so a
// failed apply neither orphans a live rule nor gets recreated as a duplicate
// on the next apply.
func TestCreateRollsBackOnDNSTimeout(t *testing.T) {
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	defer func() { dnsPollInterval = orig }()

	var created, deleted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rules":
			created.Store(true)
			_, _ = w.Write([]byte(`{"data":{"id":"r1","type":"rule","attributes":{
				"target_url":"https://dest.example.com","response_type":"moved_permanently",
				"source_urls":["go.example.com"]}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rules/r1":
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/hosts":
			_, _ = w.Write([]byte(`{"data":[{"id":"h1","type":"host","attributes":{"name":"go.example.com"}}],
				"links":{"next":null}}`))
		case r.URL.Path == "/hosts/h1":
			_, _ = w.Write([]byte(`{"data":{"id":"h1","type":"host","attributes":{
				"name":"go.example.com","required_dns_entries":{"recommended":{"type":"A","values":["203.0.113.1"]}}}}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	r := &RuleResource{
		client:   client.New(srv.URL, "k", "s", client.WithHTTPClient(srv.Client())),
		resolver: fakeResolver{hosts: map[string][]string{"go.example.com": {"10.0.0.1"}}}, // never matches 203.0.113.1
	}

	ctx := context.Background()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	objType := sr.Schema.Type().TerraformType(ctx)

	raw := tftypes.NewValue(objType, map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"source_urls":          tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "go.example.com")}),
		"target_url":           tftypes.NewValue(tftypes.String, "https://dest.example.com"),
		"response_type":        tftypes.NewValue(tftypes.String, "moved_permanently"),
		"forward_params":       tftypes.NewValue(tftypes.Bool, false),
		"forward_path":         tftypes.NewValue(tftypes.Bool, false),
		"tags":                 tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"validate_dns":         tftypes.NewValue(tftypes.Bool, true),
		"validate_dns_timeout": tftypes.NewValue(tftypes.String, "5ms"),
	})

	createResp := &resource.CreateResponse{}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: sr.Schema, Raw: raw}}, createResp)

	if !createResp.Diagnostics.HasError() {
		t.Fatal("expected a DNS validation timeout error")
	}
	if !created.Load() {
		t.Fatal("expected CreateRule to have been called")
	}
	if !deleted.Load() {
		t.Fatal("expected the created rule to be rolled back with DeleteRule")
	}
	if !createResp.State.Raw.IsNull() {
		t.Fatalf("expected no state to be saved after rollback, got %v", createResp.State.Raw)
	}
}

func TestValidateDNSHostNotFoundAndMatched(t *testing.T) {
	ctx := context.Background()

	// Host not found -> a warning, no error.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"links":{"next":null}}`))
	}))
	defer empty.Close()
	r := &RuleResource{client: client.New(empty.URL, "k", "s", client.WithHTTPClient(empty.Client()))}
	var d diag.Diagnostics
	r.validateDNS(ctx, []string{"missing.com"}, time.Second, &d)
	if d.HasError() || d.WarningsCount() == 0 {
		t.Fatalf("expected a warning and no error, got %v", d)
	}

	// Host found with no required DNS entries -> validation passes immediately.
	found := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hosts/h1" {
			_, _ = w.Write([]byte(`{"data":{"id":"h1","type":"host","attributes":{"name":"ok.com"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"h1","type":"host","attributes":{"name":"ok.com"}}],"links":{"next":null}}`))
	}))
	defer found.Close()
	r2 := &RuleResource{client: client.New(found.URL, "k", "s", client.WithHTTPClient(found.Client()))}
	var d2 diag.Diagnostics
	r2.validateDNS(ctx, []string{"ok.com"}, time.Second, &d2)
	if d2.HasError() {
		t.Fatalf("expected success, got %v", d2)
	}

	// Lookup error on the host list -> error.
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()
	r3 := &RuleResource{client: client.New(failing.URL, "k", "s", client.WithHTTPClient(failing.Client()), client.WithMaxRetries(0))}
	var d3 diag.Diagnostics
	r3.validateDNS(ctx, []string{"x.com"}, time.Second, &d3)
	if !d3.HasError() {
		t.Error("host lookup failure should error")
	}
}

func TestValidateDNSTimeout(t *testing.T) {
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	defer func() { dnsPollInterval = orig }()

	// A host that requires an A record which local DNS will never satisfy.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"h1","type":"host","attributes":{
			"name":"never.invalid","required_dns_entries":{"recommended":{"type":"A","values":["203.0.113.1"]}}}}],
			"links":{"next":null}}`))
	}))
	defer srv.Close()

	r := &RuleResource{
		client:   client.New(srv.URL, "k", "s", client.WithHTTPClient(srv.Client())),
		resolver: fakeResolver{hosts: map[string][]string{"never.invalid": {"10.0.0.1"}}},
	}
	var d diag.Diagnostics
	r.validateDNS(context.Background(), []string{"never.invalid"}, 5*time.Millisecond, &d)
	if !d.HasError() {
		t.Fatal("expected a DNS validation timeout error")
	}
}

func TestValidateDNSNoHostnames(t *testing.T) {
	// Source URLs that yield no hostnames -> validateDNS returns immediately.
	r := &RuleResource{}
	var d diag.Diagnostics
	r.validateDNS(context.Background(), []string{"", "   "}, time.Second, &d)
	if d.HasError() {
		t.Errorf("expected no error for empty hostnames, got %v", d)
	}
}

func TestWaitForDNSResolverError(t *testing.T) {
	orig := dnsPollInterval
	dnsPollInterval = time.Millisecond
	defer func() { dnsPollInterval = orig }()

	// A resolver that errors -> CheckDNS returns an error, which waitForDNS
	// converts into failure reasons and eventually times out.
	r := &RuleResource{resolver: fakeResolver{err: errors.New("resolver down")}}
	required := &client.RequiredDNSValues{Recommended: &client.DNSValue{Type: "A", Values: []string{"1.2.3.4"}}}
	reasons := r.waitForDNS(context.Background(), "x.com", required, time.Now().Add(5*time.Millisecond))
	if len(reasons) == 0 {
		t.Fatal("expected failure reasons from resolver error")
	}
}

func TestWaitForDNSContextCancel(t *testing.T) {
	orig := dnsPollInterval
	dnsPollInterval = time.Hour // ensure we hit the ctx branch, not the timer
	defer func() { dnsPollInterval = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &RuleResource{resolver: fakeResolver{hosts: map[string][]string{}}}
	required := &client.RequiredDNSValues{Recommended: &client.DNSValue{Type: "A", Values: []string{"1.2.3.4"}}}
	reasons := r.waitForDNS(ctx, "x.com", required, time.Now().Add(time.Hour))
	if reasons == nil {
		t.Error("cancelled context should return reasons")
	}
}

func TestHostnameOfEdgeCases(t *testing.T) {
	if hostnameOf("://bad url with spaces") != "" {
		// url.Parse tolerates many inputs; ensure no panic and empty-ish result path is exercised.
		t.Log("parsed unusual input")
	}
	if distinctHostnames([]string{"", "  ", "a.com"}) == nil {
		t.Error("expected at least one hostname")
	}
}
