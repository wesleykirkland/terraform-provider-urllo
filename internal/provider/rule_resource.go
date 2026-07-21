// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

const defaultValidateDNSTimeout = "5m"

// dnsPollInterval is how often local DNS is re-checked while waiting for records
// to propagate. It is a variable so tests can shorten it.
var dnsPollInterval = 10 * time.Second

var (
	_ resource.Resource                = &RuleResource{}
	_ resource.ResourceWithImportState = &RuleResource{}
)

// NewRuleResource returns a new urllo_rule resource.
func NewRuleResource() resource.Resource {
	return &RuleResource{}
}

// RuleResource implements the urllo_rule resource.
type RuleResource struct {
	client *client.Client
	// resolver is used for local DNS validation. Nil uses the system resolver.
	resolver client.DNSResolver
}

// RuleResourceModel maps urllo_rule schema data.
type RuleResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	SourceURLs         types.Set    `tfsdk:"source_urls"`
	TargetURL          types.String `tfsdk:"target_url"`
	ResponseType       types.String `tfsdk:"response_type"`
	ForwardParams      types.Bool   `tfsdk:"forward_params"`
	ForwardPath        types.Bool   `tfsdk:"forward_path"`
	Tags               types.Set    `tfsdk:"tags"`
	ValidateDNS        types.Bool   `tfsdk:"validate_dns"`
	ValidateDNSTimeout types.String `tfsdk:"validate_dns_timeout"`
}

func (r *RuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (r *RuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a redirect rule. A rule forwards one or more `source_urls` to a `target_url`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Rule identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_urls": schema.SetAttribute{
				MarkdownDescription: "URLs to redirect from, e.g. `example.com` or `example.com/path`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{
					// setvalidator would be nicer, but requiring at least one URL
					// is enforced by the API; keep the schema permissive.
				},
			},
			"target_url": schema.StringAttribute{
				MarkdownDescription: "URL to redirect to.",
				Required:            true,
			},
			"response_type": schema.StringAttribute{
				MarkdownDescription: "Redirect type: `moved_permanently` (301) or `found` (302).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(client.ResponseMovedPermanently),
				Validators: []validator.String{
					stringvalidator.OneOf(client.ResponseMovedPermanently, client.ResponseFound),
				},
			},
			"forward_params": schema.BoolAttribute{
				MarkdownDescription: "Whether request query parameters are forwarded to the target URL.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"forward_path": schema.BoolAttribute{
				MarkdownDescription: "Whether the request path is forwarded to the target URL.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags associated with the rule.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"validate_dns": schema.BoolAttribute{
				MarkdownDescription: "When `true` (default), after creating or changing the rule the provider " +
					"resolves each source host locally and waits until its DNS records match the values Urllo " +
					"requires, similar to `aws_acm_certificate_validation`. Set to `false` to skip this check.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"validate_dns_timeout": schema.StringAttribute{
				MarkdownDescription: "How long to wait for DNS to validate, as a Go duration (default `" +
					defaultValidateDNSTimeout + "`). Only used when `validate_dns` is `true`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(defaultValidateDNSTimeout),
				Validators: []validator.String{
					durationValidator{},
				},
			},
		},
	}
}

func (r *RuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		r.client = c
	}
}

func (r *RuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs := r.attributesFromModel(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.CreateRule(ctx, attrs)
	if err != nil {
		resp.Diagnostics.AddError("Error creating rule", err.Error())
		return
	}
	tflog.Trace(ctx, "created a urllo rule", map[string]any{"id": rule.ID})

	r.applyRuleToModel(ctx, rule, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.maybeValidateDNS(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetRule(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading rule", err.Error())
		return
	}
	if rule == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyRuleToModel(ctx, rule, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	attrs := r.attributesFromModel(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.UpdateRule(ctx, state.ID.ValueString(), attrs)
	if err != nil {
		resp.Diagnostics.AddError("Error updating rule", err.Error())
		return
	}

	sourcesChanged := !plan.SourceURLs.Equal(state.SourceURLs)

	r.applyRuleToModel(ctx, rule, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if sourcesChanged {
		r.maybeValidateDNS(ctx, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRule(ctx, data.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting rule", err.Error())
	}
}

func (r *RuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// attributesFromModel builds the API attribute payload from plan data.
func (r *RuleResource) attributesFromModel(ctx context.Context, data *RuleResourceModel, diags *diag.Diagnostics) client.RuleAttributes {
	return client.RuleAttributes{
		ForwardParams: data.ForwardParams.ValueBool(),
		ForwardPath:   data.ForwardPath.ValueBool(),
		ResponseType:  data.ResponseType.ValueString(),
		SourceURLs:    setToStrings(ctx, data.SourceURLs, diags),
		TargetURL:     data.TargetURL.ValueString(),
		Tags:          setToStrings(ctx, data.Tags, diags),
	}
}

// applyRuleToModel copies server-returned values into the model.
func (r *RuleResource) applyRuleToModel(ctx context.Context, rule *client.Rule, data *RuleResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(rule.ID)
	data.ResponseType = types.StringValue(rule.Attributes.ResponseType)
	data.ForwardParams = types.BoolValue(rule.Attributes.ForwardParams)
	data.ForwardPath = types.BoolValue(rule.Attributes.ForwardPath)

	// target_url and source_urls are Required (not Computed), so Terraform
	// requires the post-apply value to exactly equal what was planned. The
	// Urllo API normalizes both (e.g. appending a trailing slash to target_url,
	// reformatting source_urls), so echoing its response back here would
	// violate that contract and fail with "Provider produced inconsistent
	// result after apply". Only pull them from the API when the incoming model
	// doesn't already have them — i.e. right after import, where
	// ImportStatePassthroughID has set nothing but id. Otherwise trust the
	// already-known plan/state value.
	if data.TargetURL.IsNull() {
		data.TargetURL = types.StringValue(rule.Attributes.TargetURL)
	}
	if data.SourceURLs.IsNull() {
		data.SourceURLs = stringsToSet(ctx, rule.Attributes.SourceURLs, diags)
	}

	// Preserve a null tags value rather than an empty set to avoid perpetual
	// diffs when tags are not configured.
	if len(rule.Attributes.Tags) == 0 && data.Tags.IsNull() {
		data.Tags = types.SetNull(types.StringType)
	} else {
		data.Tags = stringsToSet(ctx, rule.Attributes.Tags, diags)
	}
}

// maybeValidateDNS runs local DNS validation when enabled on the model.
func (r *RuleResource) maybeValidateDNS(ctx context.Context, data *RuleResourceModel, diags *diag.Diagnostics) {
	if !data.ValidateDNS.ValueBool() {
		return
	}
	timeout, err := time.ParseDuration(data.ValidateDNSTimeout.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root("validate_dns_timeout"), "Invalid duration", err.Error())
		return
	}
	sources := setToStrings(ctx, data.SourceURLs, diags)
	if diags.HasError() {
		return
	}
	r.validateDNS(ctx, sources, timeout, diags)
}

// validateDNS resolves each distinct source hostname locally and waits until its
// DNS matches the values Urllo requires, or the timeout elapses.
func (r *RuleResource) validateDNS(ctx context.Context, sourceURLs []string, timeout time.Duration, diags *diag.Diagnostics) {
	hostnames := distinctHostnames(sourceURLs)
	if len(hostnames) == 0 {
		return
	}

	deadline := time.Now().Add(timeout)
	for _, hostname := range hostnames {
		host, err := r.client.GetHostByName(ctx, hostname)
		if err != nil {
			diags.AddError("Error looking up host for DNS validation",
				fmt.Sprintf("Could not look up host %q: %s", hostname, err))
			return
		}
		if host == nil {
			diags.AddWarning("Skipping DNS validation",
				fmt.Sprintf("No host named %q exists yet in Urllo, so its DNS could not be validated. "+
					"Add the host in the Urllo dashboard, or set validate_dns = false.", hostname))
			continue
		}

		lastReasons := r.waitForDNS(ctx, hostname, host.Attributes.RequiredDNSEntries, deadline)
		if lastReasons != nil {
			diags.AddError("DNS validation timed out",
				fmt.Sprintf("Host %q did not have valid DNS within %s:\n  - %s\n\n"+
					"Configure the DNS records shown in the Urllo dashboard, or set validate_dns = false.",
					hostname, timeout, strings.Join(lastReasons, "\n  - ")))
			return
		}
	}
}

// waitForDNS polls CheckDNS until it passes or the deadline is reached. It
// returns nil on success, or the most recent failure reasons on timeout.
func (r *RuleResource) waitForDNS(ctx context.Context, hostname string, required *client.RequiredDNSValues, deadline time.Time) []string {
	for {
		matched, reasons, err := client.CheckDNS(ctx, r.resolver, hostname, required)
		if err != nil {
			reasons = []string{err.Error()}
		} else if matched {
			return nil
		}

		if time.Now().After(deadline) {
			return reasons
		}
		select {
		case <-ctx.Done():
			return []string{ctx.Err().Error()}
		case <-time.After(dnsPollInterval):
		}
	}
}

// distinctHostnames extracts the unique hostnames from a set of source URLs.
func distinctHostnames(sourceURLs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range sourceURLs {
		h := hostnameOf(s)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// hostnameOf extracts the bare hostname from a source URL that may or may not
// include a scheme or path (e.g. "example.com/path" -> "example.com").
func hostnameOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "//" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
