package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &ScaffoldingProvider{}

// ScaffoldingProvider defines the provider implementation.
type ScaffoldingProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ScaffoldingProviderModel describes the provider data model.
type ScaffoldingProviderModel struct {
	ModeHost    types.String `tfsdk:"mode_host"`
	ApiToken    types.String `tfsdk:"api_token"`
	ApiSecret   types.String `tfsdk:"api_secret"`
	WorkspaceId types.String `tfsdk:"workspace_id"`
}

func (p *ScaffoldingProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "modeanalytics"
	resp.Version = p.version
}

func (p *ScaffoldingProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"mode_host": schema.StringAttribute{
				MarkdownDescription: "Mode Analytics host URL",
				Optional:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "API token for Mode Analytics",
				Optional:            true,
				Sensitive:           true,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "API secret for Mode Analytics",
				Optional:            true,
				Sensitive:           true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace ID for Mode Analytics",
				Optional:            true,
			},
		},
	}
}

func (p *ScaffoldingProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ScaffoldingProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read configuration from environment variables if not set in the provider block
	modeHost := os.Getenv("MODE_ANALYTICS_HOST")
	apiToken := os.Getenv("MODE_ANALYTICS_API_TOKEN")
	apiSecret := os.Getenv("MODE_ANALYTICS_API_SECRET")
	workspaceId := os.Getenv("MODE_ANALYTICS_WORKSPACE_ID")

	if !data.ModeHost.IsNull() {
		modeHost = data.ModeHost.ValueString()
	}

	if !data.ApiToken.IsNull() {
		apiToken = data.ApiToken.ValueString()
	}

	if !data.ApiSecret.IsNull() {
		apiSecret = data.ApiSecret.ValueString()
	}

	if !data.WorkspaceId.IsNull() {
		workspaceId = data.WorkspaceId.ValueString()
	}

	// Ensure all required configurations are set
	if modeHost == "" || apiToken == "" || apiSecret == "" || workspaceId == "" {
		resp.Diagnostics.AddError(
			"Missing Configuration",
			"All of mode_host, api_token, api_secret, and workspace_id must be set either as environment variables or in the provider configuration block.",
		)
		return
	}

	// Example client configuration for data sources and resources
	client := &http.Client{
		Transport: &customTransport{
			apiToken:            apiToken,
			apiSecret:           apiSecret,
			underlyingTransport: http.DefaultTransport,
		},
	}
	resp.DataSourceData = struct {
		Client      *http.Client
		ModeHost    string
		WorkspaceId string
	}{
		Client:      client,
		ModeHost:    modeHost,
		WorkspaceId: workspaceId,
	}
	resp.ResourceData = struct {
		Client      *http.Client
		ModeHost    string
		WorkspaceId string
	}{
		Client:      client,
		ModeHost:    modeHost,
		WorkspaceId: workspaceId,
	}
}

func (p *ScaffoldingProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGroupResource,
		NewGroupMembershipResource,
		NewDataSourcePermissionResource,
		NewCollectionResource,
		NewCollectionPermissionResource,
	}
}

func (p *ScaffoldingProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGroupDataSource,
		NewGroupMembershipsDataSource,
		NewGroupsDataSource,
		NewWorkspaceMembershipsDataSource,
		NewDataSourceDataSource,
		NewDataSourcesDataSource,
		NewCollectionDataSource,
		NewCollectionsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ScaffoldingProvider{
			version: version,
		}
	}
}

// Custom transport to add headers.
type customTransport struct {
	apiToken            string
	apiSecret           string
	underlyingTransport http.RoundTripper
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.apiToken, t.apiSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/hal+json")
	return t.underlyingTransport.RoundTrip(req)
}
