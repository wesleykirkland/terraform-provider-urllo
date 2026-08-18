// Copyright Wesley Kirkland-Daily 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

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
	// includeDNSTestedAt mirrors the provider-level include_dns_tested_at
	// setting; see its schema description for why this defaults to false.
	includeDNSTestedAt bool
}

// HostDataSourceModel maps urllo_host data-source data. It is identical to
// HostResourceModel, so the two share a single type rather than drifting.
type HostDataSourceModel = HostResourceModel

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
			"acme_enabled": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether automatic SSL is enabled."},
			"custom_404_body": dschema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Custom HTML response body served when no redirect matches, in effect only " +
					"when `not_found_action.response_code` is `404`. Null when no custom body is set.",
			},
			"match_options":      dsMatchOptionsSchema(),
			"not_found_action":   dsNotFoundActionSchema(),
			"security":           dsSecuritySchema(),
			"dns_status":         dschema.StringAttribute{Computed: true, MarkdownDescription: "DNS configuration status."},
			"certificate_status": dschema.StringAttribute{Computed: true, MarkdownDescription: "Certificate status."},
			"dns_tested_at": dschema.StringAttribute{Computed: true, MarkdownDescription: "When DNS was last " +
				"tested. Null unless the provider's `include_dns_tested_at` is set to `true`; see its schema " +
				"description for why."},
			"required_dns_entries": dsRequiredDNSSchema(),
			"detected_dns_entries": dsDetectedDNSSchema(),
		},
	}
}

func (d *HostDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if pd, ok := providerDataFrom(req.ProviderData, &resp.Diagnostics); ok {
		d.client = pd.client
		d.includeDNSTestedAt = pd.includeDNSTestedAt
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

	populateHostModel(host, &data, d.includeDNSTestedAt, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
