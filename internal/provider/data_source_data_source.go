package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DataSourceDataSource{}

func NewDataSourceDataSource() datasource.DataSource {
	return &DataSourceDataSource{}
}

// DataSourceDataSource defines the data source implementation.
type DataSourceDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type DataSourceModel struct {
	Id                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	Description               types.String `tfsdk:"description"`
	DataSourceToken           types.String `tfsdk:"data_source_token"`
	Adapter                   types.String `tfsdk:"adapter"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	UpdatedAt                 types.String `tfsdk:"updated_at"`
	HasExpensiveSchemaUpdates types.Bool   `tfsdk:"has_expensive_schema_updates"`
	Public                    types.Bool   `tfsdk:"public"`
	Asleep                    types.Bool   `tfsdk:"asleep"`
	Queryable                 types.Bool   `tfsdk:"queryable"`
	SoftDeleted               types.Bool   `tfsdk:"soft_deleted"`
	DisplayName               types.String `tfsdk:"display_name"`
	AccountId                 types.String `tfsdk:"account_id"`
	AccountUsername           types.String `tfsdk:"account_username"`
	OrganizationToken         types.String `tfsdk:"organization_token"`
	OrganizationPlanCode      types.String `tfsdk:"organization_plan_code"`
	Database                  types.String `tfsdk:"database"`
	Host                      types.String `tfsdk:"host"`
	Port                      types.Number `tfsdk:"port"`
	Ssl                       types.Bool   `tfsdk:"ssl"`
	Username                  types.String `tfsdk:"username"`
	Provider                  types.String `tfsdk:"data_source_provider"`
	Vendor                    types.String `tfsdk:"vendor"`
	Ldap                      types.Bool   `tfsdk:"ldap"`
	Warehouse                 types.String `tfsdk:"warehouse"`
	Bridged                   types.Bool   `tfsdk:"bridged"`
	AdapterVersion            types.String `tfsdk:"adapter_version"`
	CustomAttributes          types.Map    `tfsdk:"custom_attributes"`
}

func (d *DataSourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_source"
}

func (d *DataSourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Data source data source",

		Attributes: map[string]schema.Attribute{
			"data_source_token": schema.StringAttribute{
				MarkdownDescription: "Data source configurable attribute",
				Required:            true,
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"adapter": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},
			"has_expensive_schema_updates": schema.BoolAttribute{
				Computed: true,
			},
			"public": schema.BoolAttribute{
				Computed: true,
			},
			"asleep": schema.BoolAttribute{
				Computed: true,
			},
			"queryable": schema.BoolAttribute{
				Computed: true,
			},
			"soft_deleted": schema.BoolAttribute{
				Computed: true,
			},
			"display_name": schema.StringAttribute{
				Computed: true,
			},
			"account_id": schema.StringAttribute{
				Computed: true,
			},
			"account_username": schema.StringAttribute{
				Computed: true,
			},
			"organization_token": schema.StringAttribute{
				Computed: true,
			},
			"organization_plan_code": schema.StringAttribute{
				Computed: true,
			},
			"database": schema.StringAttribute{
				Computed: true,
			},
			"host": schema.StringAttribute{
				Computed: true,
			},
			"port": schema.NumberAttribute{
				Computed: true,
			},
			"ssl": schema.BoolAttribute{
				Computed: true,
			},
			"username": schema.StringAttribute{
				Computed: true,
			},
			"data_source_provider": schema.StringAttribute{
				Computed: true,
			},
			"vendor": schema.StringAttribute{
				Computed: true,
			},
			"ldap": schema.BoolAttribute{
				Computed: true,
			},
			"warehouse": schema.StringAttribute{
				Computed: true,
			},
			"bridged": schema.BoolAttribute{
				Computed: true,
			},
			"adapter_version": schema.StringAttribute{
				Computed: true,
			},
			"custom_attributes": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *DataSourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(struct {
		Client      *http.Client
		ModeHost    string
		WorkspaceId string
	})

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected struct with *http.Client, ModeHost, and WorkspaceId, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = config.Client
	d.modeHost = config.ModeHost
	d.workspaceId = config.WorkspaceId
}

func (d *DataSourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Construct the URL using modeHost, workspaceId, and groupToken
	url := fmt.Sprintf("%s/api/%s/data_sources/%s", d.modeHost, d.workspaceId, data.DataSourceToken.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %s", err))
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	httpResp, err := HttpRetry(d.client, httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unexpected status code: %d", httpResp.StatusCode))
		return
	}

	// Parse the response body
	var responseData struct {
		Id                        string            `json:"id"`
		Name                      string            `json:"name"`
		Description               string            `json:"description"`
		DataSourceToken           string            `json:"token"`
		Adapter                   string            `json:"adapter"`
		CreatedAt                 string            `json:"created_at"`
		UpdatedAt                 string            `json:"updated_at"`
		HasExpensiveSchemaUpdates bool              `json:"has_expensive_schema_updates"`
		Public                    bool              `json:"public"`
		Asleep                    bool              `json:"asleep"`
		Queryable                 bool              `json:"queryable"`
		SoftDeleted               bool              `json:"soft_deleted"`
		DisplayName               string            `json:"display_name"`
		AccountId                 string            `json:"account_id"`
		AccountUsername           string            `json:"account_username"`
		OrganizationToken         string            `json:"organization_token"`
		OrganizationPlanCode      string            `json:"organization_plan_code"`
		Database                  string            `json:"database"`
		Host                      string            `json:"host"`
		Port                      float64           `json:"port"`
		Ssl                       bool              `json:"ssl"`
		Username                  string            `json:"username"`
		Provider                  string            `json:"provider"`
		Vendor                    string            `json:"vendor"`
		Ldap                      bool              `json:"ldap"`
		Warehouse                 string            `json:"warehouse"`
		Bridged                   bool              `json:"bridged"`
		AdapterVersion            string            `json:"adapter_version"`
		CustomAttributes          map[string]string `json:"custom_attributes"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	// Assign the parsed values to the data model
	data.Id = types.StringValue(responseData.Id)
	data.Name = types.StringValue(responseData.Name)
	data.Description = types.StringValue(responseData.Description)
	data.DataSourceToken = types.StringValue(responseData.DataSourceToken)
	data.Adapter = types.StringValue(responseData.Adapter)
	data.CreatedAt = types.StringValue(responseData.CreatedAt)
	data.UpdatedAt = types.StringValue(responseData.UpdatedAt)
	data.HasExpensiveSchemaUpdates = types.BoolValue(responseData.HasExpensiveSchemaUpdates)
	data.Public = types.BoolValue(responseData.Public)
	data.Asleep = types.BoolValue(responseData.Asleep)
	data.Queryable = types.BoolValue(responseData.Queryable)
	data.SoftDeleted = types.BoolValue(responseData.SoftDeleted)
	data.DisplayName = types.StringValue(responseData.DisplayName)
	data.AccountId = types.StringValue(responseData.AccountId)
	data.AccountUsername = types.StringValue(responseData.AccountUsername)
	data.OrganizationToken = types.StringValue(responseData.OrganizationToken)
	data.OrganizationPlanCode = types.StringValue(responseData.OrganizationPlanCode)
	data.Database = types.StringValue(responseData.Database)
	data.Host = types.StringValue(responseData.Host)
	data.Port = types.NumberValue(big.NewFloat(responseData.Port))
	data.Ssl = types.BoolValue(responseData.Ssl)
	data.Username = types.StringValue(responseData.Username)
	data.Provider = types.StringValue(responseData.Provider)
	data.Vendor = types.StringValue(responseData.Vendor)
	data.Ldap = types.BoolValue(responseData.Ldap)
	data.Warehouse = types.StringValue(responseData.Warehouse)
	data.Bridged = types.BoolValue(responseData.Bridged)
	data.AdapterVersion = types.StringValue(responseData.AdapterVersion)
	data.CustomAttributes, _ = types.MapValueFrom(ctx, types.StringType, responseData.CustomAttributes)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
