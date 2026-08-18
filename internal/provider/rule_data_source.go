// Copyright (c) Wesley Kirkland-Daily
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	ID                 types.String `tfsdk:"id"`
	SourceURLs         types.Set    `tfsdk:"source_urls"`
	TargetURL          types.String `tfsdk:"target_url"`
	ResponseType       types.String `tfsdk:"response_type"`
	ForwardParams      types.Bool   `tfsdk:"forward_params"`
	ForwardPath        types.Bool   `tfsdk:"forward_path"`
	Tags               types.Set    `tfsdk:"tags"`
	Name               types.String `tfsdk:"name"`
	DNSStatus          types.String `tfsdk:"dns_status"`
	CertificateStatus  types.String `tfsdk:"certificate_status"`
	IncludeAnalytics   types.Bool   `tfsdk:"include_analytics"`
	AnalyticsStartDate types.String `tfsdk:"analytics_start_date"`
	AnalyticsEndDate   types.String `tfsdk:"analytics_end_date"`
	Analytics          types.Object `tfsdk:"analytics"`
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
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name Urllo assigns to the rule.",
				Computed:            true,
			},
			"dns_status": schema.StringAttribute{
				MarkdownDescription: "DNS configuration status of the rule's source host.",
				Computed:            true,
			},
			"certificate_status": schema.StringAttribute{
				MarkdownDescription: "Certificate status of the rule's source host.",
				Computed:            true,
			},
			"include_analytics": schema.BoolAttribute{
				MarkdownDescription: "Whether to fetch request-volume analytics for the rule (the API " +
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
			"analytics": schema.SingleNestedAttribute{
				MarkdownDescription: "Request-volume analytics for the rule. Null unless `include_analytics` " +
					"is `true`.",
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"analytics_start_date": schema.StringAttribute{
						MarkdownDescription: "Effective start date used for the analytics data (may differ " +
							"from the requested date if clamped by your plan).",
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

	opts := client.ListRulesOptions{
		IncludeAnalytics:   data.IncludeAnalytics.ValueBool(),
		AnalyticsStartDate: data.AnalyticsStartDate.ValueString(),
		AnalyticsEndDate:   data.AnalyticsEndDate.ValueString(),
	}

	rule, err := d.client.GetRule(ctx, data.ID.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Error reading rule", err.Error())
		return
	}
	if rule == nil {
		resp.Diagnostics.AddError("Rule not found", fmt.Sprintf("No rule with id %q was found.", data.ID.ValueString()))
		return
	}

	data.ID = types.StringValue(rule.ID)
	data.TargetURL = types.StringValue(rule.Attributes.TargetURL)
	data.ResponseType = types.StringValue(rule.Attributes.ResponseType)
	data.ForwardParams = types.BoolValue(rule.Attributes.ForwardParams)
	data.ForwardPath = types.BoolValue(rule.Attributes.ForwardPath)
	data.SourceURLs = stringsToSet(ctx, rule.Attributes.SourceURLs, &resp.Diagnostics)
	data.Tags = stringsToSet(ctx, rule.Attributes.Tags, &resp.Diagnostics)
	data.Name = types.StringValue(rule.Attributes.Name)
	data.DNSStatus = types.StringValue(rule.Attributes.DNSStatus)
	data.CertificateStatus = types.StringValue(rule.Attributes.CertificateStatus)
	data.Analytics = analyticsToObject(rule.Attributes.Analytics, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
