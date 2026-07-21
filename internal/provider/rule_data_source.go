// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

var _ datasource.DataSource = &RuleDataSource{}

// NewRuleDataSource returns a new urllo_rule data source.
func NewRuleDataSource() datasource.DataSource {
	return &RuleDataSource{}
}

// RuleDataSource looks up a single rule by ID.
type RuleDataSource struct {
	client *client.Client
}

// RuleDataSourceModel maps urllo_rule data-source data.
type RuleDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	SourceURLs    types.Set    `tfsdk:"source_urls"`
	TargetURL     types.String `tfsdk:"target_url"`
	ResponseType  types.String `tfsdk:"response_type"`
	ForwardParams types.Bool   `tfsdk:"forward_params"`
	ForwardPath   types.Bool   `tfsdk:"forward_path"`
	Tags          types.Set    `tfsdk:"tags"`
}

func (d *RuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (d *RuleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single redirect rule by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Rule identifier.",
				Required:            true,
			},
			"source_urls": schema.SetAttribute{
				MarkdownDescription: "URLs the rule redirects from.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"target_url": schema.StringAttribute{
				MarkdownDescription: "URL the rule redirects to.",
				Computed:            true,
			},
			"response_type": schema.StringAttribute{
				MarkdownDescription: "Redirect type.",
				Computed:            true,
			},
			"forward_params": schema.BoolAttribute{
				MarkdownDescription: "Whether query parameters are forwarded.",
				Computed:            true,
			},
			"forward_path": schema.BoolAttribute{
				MarkdownDescription: "Whether the path is forwarded.",
				Computed:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Tags associated with the rule.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *RuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		d.client = c
	}
}

func (d *RuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := d.client.GetRule(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading rule", err.Error())
		return
	}

	data.ID = types.StringValue(rule.ID)
	data.TargetURL = types.StringValue(rule.Attributes.TargetURL)
	data.ResponseType = types.StringValue(rule.Attributes.ResponseType)
	data.ForwardParams = types.BoolValue(rule.Attributes.ForwardParams)
	data.ForwardPath = types.BoolValue(rule.Attributes.ForwardPath)
	data.SourceURLs = stringsToSet(ctx, rule.Attributes.SourceURLs, &resp.Diagnostics)
	data.Tags = stringsToSet(ctx, rule.Attributes.Tags, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
