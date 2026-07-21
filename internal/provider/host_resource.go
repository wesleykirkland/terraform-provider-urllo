// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

var (
	_ resource.Resource                = &HostResource{}
	_ resource.ResourceWithImportState = &HostResource{}
)

// NewHostResource returns a new urllo_host resource.
func NewHostResource() resource.Resource {
	return &HostResource{}
}

// HostResource manages settings of an existing Urllo host. Hosts are provisioned
// via DNS in the Urllo dashboard, so this resource adopts an existing host by
// name and manages its writable settings. Destroying the resource only removes
// it from Terraform state; the host itself is not deleted.
type HostResource struct {
	client *client.Client
}

// HostResourceModel maps urllo_host schema data. Nested settings are held as
// types.Object so null/unknown states are handled explicitly.
type HostResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	ACMEEnabled        types.Bool   `tfsdk:"acme_enabled"`
	Custom404Body      types.String `tfsdk:"custom_404_body"`
	MatchOptions       types.Object `tfsdk:"match_options"`
	NotFoundAction     types.Object `tfsdk:"not_found_action"`
	Security           types.Object `tfsdk:"security"`
	DNSStatus          types.String `tfsdk:"dns_status"`
	CertificateStatus  types.String `tfsdk:"certificate_status"`
	DNSTestedAt        types.String `tfsdk:"dns_tested_at"`
	RequiredDNSEntries types.Object `tfsdk:"required_dns_entries"`
	DetectedDNSEntries types.List   `tfsdk:"detected_dns_entries"`
}

func (r *HostResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *HostResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the settings of an existing source host. Hosts are created by adding a " +
			"domain in the Urllo dashboard and configuring DNS; this resource adopts a host by `name` and " +
			"manages its writable settings. Destroying the resource removes it from Terraform state only — the " +
			"host is not deleted from Urllo.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Host identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The hostname to manage, e.g. `www.example.com`. The host must already " +
					"exist in Urllo. Changing this adopts a different host.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"acme_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether automatic SSL certificate provisioning is enabled.",
				Optional:            true,
				Computed:            true,
			},
			"custom_404_body": schema.StringAttribute{
				MarkdownDescription: "Custom response body served when no redirect matches. Write-only: the API " +
					"does not return the stored body, so this value is not refreshed from Urllo.",
				Optional:  true,
				Sensitive: true,
			},
			"match_options": schema.SingleNestedAttribute{
				MarkdownDescription: "How source URLs are matched.",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"case_insensitive": schema.BoolAttribute{
						MarkdownDescription: "Ignore case when matching paths and query parameters.",
						Optional:            true,
						Computed:            true,
					},
					"slash_insensitive": schema.BoolAttribute{
						MarkdownDescription: "Ignore trailing forward slashes on paths.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"not_found_action": schema.SingleNestedAttribute{
				MarkdownDescription: "Behaviour when no matching redirect is found.",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"forward_params": schema.BoolAttribute{
						MarkdownDescription: "Copy source query parameters to the target URL.",
						Optional:            true,
						Computed:            true,
					},
					"forward_path": schema.BoolAttribute{
						MarkdownDescription: "Copy the source path to the target URL.",
						Optional:            true,
						Computed:            true,
					},
					"response_code": schema.Int64Attribute{
						MarkdownDescription: "Response code when no match is found: 301, 302, or 404.",
						Optional:            true,
						Computed:            true,
						Validators:          []validator.Int64{int64validator.OneOf(301, 302, 404)},
					},
					"response_url": schema.StringAttribute{
						MarkdownDescription: "Redirect target when `response_code` is 301 or 302 and no match is found.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"security": schema.SingleNestedAttribute{
				MarkdownDescription: "HTTPS and HSTS security settings.",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"https_upgrade": schema.BoolAttribute{
						MarkdownDescription: "Upgrade HTTP requests to HTTPS.",
						Optional:            true,
						Computed:            true,
					},
					"prevent_foreign_embedding": schema.BoolAttribute{
						MarkdownDescription: "Prevent foreign embedding and JavaScript.",
						Optional:            true,
						Computed:            true,
					},
					"hsts_include_sub_domains": schema.BoolAttribute{
						MarkdownDescription: "Apply HSTS to all subdomains.",
						Optional:            true,
						Computed:            true,
					},
					"hsts_max_age": schema.Int64Attribute{
						MarkdownDescription: "HSTS max-age in seconds.",
						Optional:            true,
						Computed:            true,
					},
					"hsts_preload": schema.BoolAttribute{
						MarkdownDescription: "Include the preload directive in HSTS headers.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"dns_status": schema.StringAttribute{
				MarkdownDescription: "DNS configuration status: `active`, `invalid`, or `requires_verification`.",
				Computed:            true,
			},
			"certificate_status": schema.StringAttribute{
				MarkdownDescription: "Current certificate status.",
				Computed:            true,
			},
			"dns_tested_at": schema.StringAttribute{
				MarkdownDescription: "When the host's DNS was last tested.",
				Computed:            true,
			},
			"required_dns_entries": requiredDNSSchema(),
			"detected_dns_entries": detectedDNSSchema(),
		},
	}
}

func (r *HostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		r.client = c
	}
}

func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host, err := r.client.GetHostByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error looking up host", err.Error())
		return
	}
	if host == nil {
		resp.Diagnostics.AddError("Host not found",
			fmt.Sprintf("No host named %q exists in Urllo. Add the domain in the Urllo dashboard first.", data.Name.ValueString()))
		return
	}

	updated := r.applyUpdate(ctx, host.ID, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "adopted a urllo host", map[string]any{"id": updated.ID})

	r.applyHostToModel(updated, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host, err := r.readHost(ctx, &data)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading host", err.Error())
		return
	}
	if host == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyHostToModel(host, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	updated := r.applyUpdate(ctx, state.ID.ValueString(), &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyHostToModel(updated, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Hosts cannot be deleted through the API; they are provisioned via DNS.
	// Removing the resource just drops it from state.
	resp.Diagnostics.AddWarning("Host not deleted",
		"The Urllo API does not support deleting hosts. The host has been removed from Terraform state only; "+
			"it still exists in Urllo. Remove the domain from the Urllo dashboard to delete it.")
}

func (r *HostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readHost fetches the host by ID when known, otherwise by name.
func (r *HostResource) readHost(ctx context.Context, data *HostResourceModel) (*client.Host, error) {
	if id := data.ID.ValueString(); id != "" {
		return r.client.GetHost(ctx, id)
	}
	return r.client.GetHostByName(ctx, data.Name.ValueString())
}

// applyUpdate builds the write payload from the plan and PATCHes the host.
func (r *HostResource) applyUpdate(ctx context.Context, id string, data *HostResourceModel, diags *diag.Diagnostics) *client.Host {
	upd := client.HostUpdate{
		MatchOptions:   objectToMatchOptions(ctx, data.MatchOptions, diags),
		NotFoundAction: objectToNotFoundAction(ctx, data.NotFoundAction, diags),
		Security:       objectToSecurity(ctx, data.Security, diags),
		Custom404Body:  stringToPtr(data.Custom404Body),
	}
	if !data.ACMEEnabled.IsNull() && !data.ACMEEnabled.IsUnknown() {
		v := data.ACMEEnabled.ValueBool()
		upd.ACMEEnabled = &v
	}
	if diags.HasError() {
		return nil
	}

	host, err := r.client.UpdateHost(ctx, id, upd)
	if err != nil {
		diags.AddError("Error updating host", err.Error())
		return nil
	}
	return host
}

// applyHostToModel copies server-returned values into the model. Custom404Body
// is preserved as-is since the API never returns it.
func (r *HostResource) applyHostToModel(host *client.Host, data *HostResourceModel, diags *diag.Diagnostics) {
	a := host.Attributes
	data.ID = types.StringValue(host.ID)
	data.Name = types.StringValue(a.Name)
	data.ACMEEnabled = types.BoolValue(a.ACMEEnabled)
	data.DNSStatus = types.StringValue(a.DNSStatus)
	data.CertificateStatus = types.StringValue(a.CertificateStatus)
	data.DNSTestedAt = stringPtrValue(a.DNSTestedAt)
	data.MatchOptions = matchOptionsToObject(a.MatchOptions, diags)
	data.NotFoundAction = notFoundActionToObject(a.NotFoundAction, diags)
	data.Security = securityToObject(a.Security, diags)
	data.RequiredDNSEntries = requiredDNSToObject(a.RequiredDNSEntries, diags)
	data.DetectedDNSEntries = detectedDNSToList(a.DetectedDNSEntries, diags)
}
