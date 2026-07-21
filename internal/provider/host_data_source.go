// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

var _ datasource.DataSource = &HostDataSource{}

// NewHostDataSource returns a new urllo_host data source.
func NewHostDataSource() datasource.DataSource {
	return &HostDataSource{}
}

// HostDataSource looks up a single host by ID or name.
type HostDataSource struct {
	client *client.Client
}

// HostDataSourceModel maps urllo_host data-source data.
type HostDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	ACMEEnabled        types.Bool   `tfsdk:"acme_enabled"`
	MatchOptions       types.Object `tfsdk:"match_options"`
	NotFoundAction     types.Object `tfsdk:"not_found_action"`
	Security           types.Object `tfsdk:"security"`
	DNSStatus          types.String `tfsdk:"dns_status"`
	CertificateStatus  types.String `tfsdk:"certificate_status"`
	DNSTestedAt        types.String `tfsdk:"dns_tested_at"`
	RequiredDNSEntries types.Object `tfsdk:"required_dns_entries"`
	DetectedDNSEntries types.List   `tfsdk:"detected_dns_entries"`
}

func (d *HostDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (d *HostDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Fetches a single source host by `id` or `name`.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				MarkdownDescription: "Host identifier. One of `id` or `name` is required.",
				Optional:            true,
				Computed:            true,
			},
			"name": dschema.StringAttribute{
				MarkdownDescription: "Hostname. One of `id` or `name` is required.",
				Optional:            true,
				Computed:            true,
			},
			"acme_enabled":         dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether automatic SSL is enabled."},
			"match_options":        dsMatchOptionsSchema(),
			"not_found_action":     dsNotFoundActionSchema(),
			"security":             dsSecuritySchema(),
			"dns_status":           dschema.StringAttribute{Computed: true, MarkdownDescription: "DNS configuration status."},
			"certificate_status":   dschema.StringAttribute{Computed: true, MarkdownDescription: "Certificate status."},
			"dns_tested_at":        dschema.StringAttribute{Computed: true, MarkdownDescription: "When DNS was last tested."},
			"required_dns_entries": dsRequiredDNSSchema(),
			"detected_dns_entries": dsDetectedDNSSchema(),
		},
	}
}

func (d *HostDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		d.client = c
	}
}

func (d *HostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data HostDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.ValueString() == "" && data.Name.ValueString() == "" {
		resp.Diagnostics.AddError("Missing host selector", "Set either `id` or `name` to look up a host.")
		return
	}

	var (
		host *client.Host
		err  error
	)
	if data.ID.ValueString() != "" {
		host, err = d.client.GetHost(ctx, data.ID.ValueString())
	} else {
		host, err = d.client.GetHostByName(ctx, data.Name.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading host", err.Error())
		return
	}
	if host == nil {
		resp.Diagnostics.AddError("Host not found", "No host matched the given `name`.")
		return
	}

	a := host.Attributes
	data.ID = types.StringValue(host.ID)
	data.Name = types.StringValue(a.Name)
	data.ACMEEnabled = types.BoolValue(a.ACMEEnabled)
	data.DNSStatus = types.StringValue(a.DNSStatus)
	data.CertificateStatus = types.StringValue(a.CertificateStatus)
	data.DNSTestedAt = stringPtrValue(a.DNSTestedAt)
	data.MatchOptions = matchOptionsToObject(a.MatchOptions, &resp.Diagnostics)
	data.NotFoundAction = notFoundActionToObject(a.NotFoundAction, &resp.Diagnostics)
	data.Security = securityToObject(a.Security, &resp.Diagnostics)
	data.RequiredDNSEntries = requiredDNSToObject(a.RequiredDNSEntries, &resp.Diagnostics)
	data.DetectedDNSEntries = detectedDNSToList(a.DetectedDNSEntries, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
