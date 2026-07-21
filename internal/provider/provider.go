// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/wesleykirkland/terraform-provider-urllo/internal/client"
)

// Environment variables used as fallbacks for provider configuration.
const (
	envAPIKey    = "URLLO_API_KEY"
	envAPISecret = "URLLO_API_SECRET"
	envEndpoint  = "URLLO_ENDPOINT"
)

const (
	// envVarSuffixMD closes the markdown code span opened before an env var name.
	envVarSuffixMD = "` environment variable."
	envVarSuffix   = " environment variable."
)

// Ensure UrlloProvider satisfies the provider interface.
var _ provider.Provider = &UrlloProvider{}

// UrlloProvider is the Urllo Terraform provider.
type UrlloProvider struct {
	// version is set to the provider version on release, "dev" when built and
	// run locally, and "test" during acceptance testing.
	version string
}

// UrlloProviderModel maps provider configuration.
type UrlloProviderModel struct {
	APIKey    types.String `tfsdk:"api_key"`
	APISecret types.String `tfsdk:"api_secret"`
	Endpoint  types.String `tfsdk:"endpoint"`
}

func (p *UrlloProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "urllo"
	resp.Version = p.version
}

func (p *UrlloProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Urllo provider manages redirect rules and source hosts in the " +
			"[Urllo](https://urllo.com) redirection service.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Urllo API key (HTTP Basic username). May also be set with the `" +
					envAPIKey + envVarSuffixMD,
				Optional: true,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "Urllo API secret (HTTP Basic password). May also be set with the `" +
					envAPISecret + envVarSuffixMD,
				Optional:  true,
				Sensitive: true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL for the Urllo API. Defaults to `" + client.DefaultBaseURL +
					"`. May also be set with the `" + envEndpoint + envVarSuffixMD,
				Optional: true,
			},
		},
	}
}

func (p *UrlloProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data UrlloProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Values that are still unknown at plan time cannot be resolved yet.
	if data.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Unknown Urllo API key",
			"The api_key value is unknown. Set a static value or the "+envAPIKey+envVarSuffix)
	}
	if data.APISecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_secret"), "Unknown Urllo API secret",
			"The api_secret value is unknown. Set a static value or the "+envAPISecret+envVarSuffix)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolution order: explicit config value, then environment variable, then
	// default (for the endpoint only).
	apiKey := firstNonEmpty(data.APIKey.ValueString(), os.Getenv(envAPIKey))
	apiSecret := firstNonEmpty(data.APISecret.ValueString(), os.Getenv(envAPISecret))
	endpoint := firstNonEmpty(data.Endpoint.ValueString(), os.Getenv(envEndpoint), client.DefaultBaseURL)

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing Urllo API key",
			"Set the api_key provider attribute or the "+envAPIKey+envVarSuffix)
	}
	if apiSecret == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_secret"), "Missing Urllo API secret",
			"Set the api_secret provider attribute or the "+envAPISecret+envVarSuffix)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	ua := "terraform-provider-urllo/" + p.version
	c := client.New(endpoint, apiKey, apiSecret, client.WithUserAgent(ua))
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *UrlloProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRuleResource,
		NewHostResource,
	}
}

func (p *UrlloProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRuleDataSource,
		NewRulesDataSource,
		NewHostDataSource,
		NewHostsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &UrlloProvider{version: version}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
