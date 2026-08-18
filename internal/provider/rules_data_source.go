// Copyright Wesley Kirkland-Daily 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// listRulesAPIDocsLink points at the API reference for the endpoint this data
// source calls, so schema descriptions can cite it without repeating the URL.
const listRulesAPIDocsLink = "[List Rules API docs](https://dashboard.urllo.com/docs/api#tag/Rules/operation/listRules)"

var _ datasource.DataSource = &RulesDataSource{}

// NewRulesDataSource returns a new urllo_rules data source.
func NewRulesDataSource() datasource.DataSource {
	return &RulesDataSource{}
}

// RulesDataSource lists redirect rules with optional filters.
type RulesDataSource struct {
	client *client.Client
}

// RulesDataSourceModel maps urllo_rules data-source data.
type RulesDataSourceModel struct {
	SourceQuery        types.String `tfsdk:"source_query"`
	TargetQuery        types.String `tfsdk:"target_query"`
	Tags               types.Set    `tfsdk:"tags"`
	TagMatchStrategy   types.String `tfsdk:"tag_match_strategy"`
	IncludeAnalytics   types.Bool   `tfsdk:"include_analytics"`
	AnalyticsStartDate types.String `tfsdk:"analytics_start_date"`
	AnalyticsEndDate   types.String `tfsdk:"analytics_end_date"`
	Rules              types.List   `tfsdk:"rules"`
}

func (d *RulesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rules"
}

func (d *RulesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists redirect rules, optionally filtered by source/target URL and tags. See the " +
			listRulesAPIDocsLink + " for parameter semantics.",
		Attributes: map[string]schema.Attribute{
			"source_query": schema.StringAttribute{
				MarkdownDescription: "Filter by source URL (the API `sq` parameter). See the " + listRulesAPIDocsLink + ".",
				Optional:            true,
			},
			"target_query": schema.StringAttribute{
				MarkdownDescription: "Filter by target URL (the API `tq` parameter). See the " + listRulesAPIDocsLink + ".",
				Optional:            true,
			},
			"tags": schema.SetAttribute{
				MarkdownDescription: "Filter by tags (the API `tags[]` parameter). See the " + listRulesAPIDocsLink + ".",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"tag_match_strategy": schema.StringAttribute{
				MarkdownDescription: "How tags are matched: `any` (default) or `all` (the API `tag_match_strategy` " +
					"parameter). See the " + listRulesAPIDocsLink + ".",
				Optional:   true,
				Validators: []validator.String{stringvalidator.OneOf("any", "all")},
			},
			"include_analytics": schema.BoolAttribute{
				MarkdownDescription: "Whether to fetch request-volume analytics for each rule (the API " +
					"`include_analytics` parameter). Defaults to `false`. Note that `requests_processed` " +
					"changes over time, so setting this to `true` will show plan drift on every run for any " +
					"rule receiving traffic.",
				Optional: true,
			},
			"analytics_start_date": schema.StringAttribute{
				MarkdownDescription: "Start date (`YYYY-MM-DD`) for analytics data, when `include_analytics` " +
					"is `true`. Defaults to one month ago. Ignored otherwise.",
				Optional:   true,
				Validators: []validator.String{analyticsDateValidator},
			},
			"analytics_end_date": schema.StringAttribute{
				MarkdownDescription: "End date (`YYYY-MM-DD`) for analytics data, when `include_analytics` " +
					"is `true`. Defaults to yesterday. Ignored otherwise.",
				Optional:   true,
				Validators: []validator.String{analyticsDateValidator},
			},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "The matching rules.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Rule identifier."},
						"source_urls":        schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Source URLs."},
						"target_url":         schema.StringAttribute{Computed: true, MarkdownDescription: "Target URL."},
						"response_type":      schema.StringAttribute{Computed: true, MarkdownDescription: "Redirect type."},
						"forward_params":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether query parameters are forwarded."},
						"forward_path":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the path is forwarded."},
						"tags":               schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Tags."},
						"name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Display name Urllo assigns to the rule."},
						"dns_status":         schema.StringAttribute{Computed: true, MarkdownDescription: "DNS configuration status of the rule's source host."},
						"certificate_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Certificate status of the rule's source host."},
						"analytics": schema.SingleNestedAttribute{
							MarkdownDescription: "Request-volume analytics for the rule. Null unless " +
								"`include_analytics` is `true`.",
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"analytics_start_date": schema.StringAttribute{
									MarkdownDescription: "Effective start date used for the analytics data " +
										"(may differ from the requested date if clamped by your plan).",
									Computed: true,
								},
								"analytics_end_date": schema.StringAttribute{
									MarkdownDescription: "Effective end date used for the analytics data.",
									Computed:            true,
								},
								"requests_processed": schema.Int64Attribute{
									MarkdownDescription: "Number of requests processed for the rule during the date range.",
									Computed:            true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *RulesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics); ok {
		d.client = c
	}
}

func (d *RulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RulesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.ListRulesOptions{
		SourceQuery:        data.SourceQuery.ValueString(),
		TargetQuery:        data.TargetQuery.ValueString(),
		Tags:               setToStrings(ctx, data.Tags, &resp.Diagnostics),
		TagMatchStrategy:   data.TagMatchStrategy.ValueString(),
		IncludeAnalytics:   data.IncludeAnalytics.ValueBool(),
		AnalyticsStartDate: data.AnalyticsStartDate.ValueString(),
		AnalyticsEndDate:   data.AnalyticsEndDate.ValueString(),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	rules, err := d.client.ListRules(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing rules", err.Error())
		return
	}

	data.Rules = rulesToList(ctx, rules, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
