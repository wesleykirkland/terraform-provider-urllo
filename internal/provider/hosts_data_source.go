// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

var _ datasource.DataSource = &HostsDataSource{}

// NewHostsDataSource returns a new urllo_hosts data source.
func NewHostsDataSource() datasource.DataSource {
	return &HostsDataSource{}
}

// HostsDataSource lists all source hosts.
type HostsDataSource struct {
	client *client.Client
}

// HostsDataSourceModel maps urllo_hosts data-source data.
type HostsDataSourceModel struct {
	Hosts types.List `tfsdk:"hosts"`
}

var hostSummaryAttrTypes = map[string]attr.Type{
	"id":                 types.StringType,
	"name":               types.StringType,
	"dns_status":         types.StringType,
	"certificate_status": types.StringType,
}

func (d *HostsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hosts"
}

func (d *HostsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all source hosts.",
		Attributes: map[string]schema.Attribute{
			"hosts": schema.ListNestedAttribute{
				MarkdownDescription: "The source hosts.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Host identifier."},
						"name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Hostname."},
						"dns_status":         schema.StringAttribute{Computed: true, MarkdownDescription: "DNS status."},
						"certificate_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Certificate status."},
					},
				},
			},
		},
	}
}

func (d *HostsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		d.client = c
	}
}

func (d *HostsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	hosts, err := d.client.ListHosts(ctx, 100)
	if err != nil {
		resp.Diagnostics.AddError("Error listing hosts", err.Error())
		return
	}

	elemType := types.ObjectType{AttrTypes: hostSummaryAttrTypes}
	objs := make([]attr.Value, 0, len(hosts))
	for _, h := range hosts {
		obj, d := types.ObjectValue(hostSummaryAttrTypes, map[string]attr.Value{
			"id":                 types.StringValue(h.ID),
			"name":               types.StringValue(h.Attributes.Name),
			"dns_status":         types.StringValue(h.Attributes.DNSStatus),
			"certificate_status": types.StringValue(h.Attributes.CertificateStatus),
		})
		resp.Diagnostics.Append(d...)
		objs = append(objs, obj)
	}

	list, diags := types.ListValue(elemType, objs)
	resp.Diagnostics.Append(diags...)

	var data HostsDataSourceModel
	data.Hosts = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
