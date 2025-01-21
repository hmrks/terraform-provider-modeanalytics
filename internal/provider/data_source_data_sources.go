package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DataSourcesDataSource{}

func NewDataSourcesDataSource() datasource.DataSource {
	return &DataSourcesDataSource{}
}

type DataSourcesDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type DataSourcesDataSourceModel struct {
	DataSources []DataSourceModel `tfsdk:"data_sources"`
}

func (d *DataSourcesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_sources"
}

func (d *DataSourcesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data sources data source",

		Attributes: map[string]schema.Attribute{
			"data_sources": schema.ListAttribute{
				MarkdownDescription: "List of data sources",
				Computed:            true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"id":                           types.StringType,
						"name":                         types.StringType,
						"description":                  types.StringType,
						"data_source_token":            types.StringType,
						"adapter":                      types.StringType,
						"created_at":                   types.StringType,
						"updated_at":                   types.StringType,
						"has_expensive_schema_updates": types.BoolType,
						"public":                       types.BoolType,
						"asleep":                       types.BoolType,
						"queryable":                    types.BoolType,
						"soft_deleted":                 types.BoolType,
						"display_name":                 types.StringType,
						"account_id":                   types.StringType,
						"account_username":             types.StringType,
						"organization_token":           types.StringType,
						"organization_plan_code":       types.StringType,
						"database":                     types.StringType,
						"host":                         types.StringType,
						"port":                         types.NumberType,
						"ssl":                          types.BoolType,
						"username":                     types.StringType,
						"data_source_provider":         types.StringType,
						"vendor":                       types.StringType,
						"ldap":                         types.BoolType,
						"warehouse":                    types.StringType,
						"bridged":                      types.BoolType,
						"adapter_version":              types.StringType,
						"custom_attributes":            types.MapType{ElemType: types.StringType},
					},
				},
			},
		},
	}
}

func (d *DataSourcesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
			fmt.Sprintf("Expected struct with *http.Client, got %T", req.ProviderData),
		)
		return
	}

	d.client = config.Client
	d.modeHost = config.ModeHost
	d.workspaceId = config.WorkspaceId
}

func (d *DataSourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataSourcesDataSourceModel

	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/data_sources", d.modeHost, d.workspaceId)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %s", err))
		return
	}

	httpResp, err := HttpRetry(d.client, httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list data sources: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list data sources: %d", httpResp.StatusCode))
		return
	}

	// Parse the response body
	var responseData struct {
		Embedded struct {
			DataSources []struct {
				Id                        string                 `json:"id"`
				Name                      string                 `json:"name"`
				Description               string                 `json:"description"`
				DataSourceToken           string                 `json:"token"`
				Adapter                   string                 `json:"adapter"`
				CreatedAt                 string                 `json:"created_at"`
				UpdatedAt                 string                 `json:"updated_at"`
				HasExpensiveSchemaUpdates bool                   `json:"has_expensive_schema_updates"`
				Public                    bool                   `json:"public"`
				Asleep                    bool                   `json:"asleep"`
				Queryable                 bool                   `json:"queryable"`
				SoftDeleted               bool                   `json:"soft_deleted"`
				DisplayName               string                 `json:"display_name"`
				AccountId                 string                 `json:"account_id"`
				AccountUsername           string                 `json:"account_username"`
				OrganizationToken         string                 `json:"organization_token"`
				OrganizationPlanCode      string                 `json:"organization_plan_code"`
				Database                  string                 `json:"database"`
				Host                      string                 `json:"host"`
				Port                      float64                `json:"port"`
				Ssl                       bool                   `json:"ssl"`
				Username                  string                 `json:"username"`
				Provider                  string                 `json:"provider"`
				Vendor                    string                 `json:"vendor"`
				Ldap                      bool                   `json:"ldap"`
				Warehouse                 string                 `json:"warehouse"`
				Bridged                   bool                   `json:"bridged"`
				AdapterVersion            string                 `json:"adapter_version"`
				CustomAttributes          map[string]interface{} `json:"custom_attributes"`
			} `json:"data_sources"`
		} `json:"_embedded"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	data.DataSources = []DataSourceModel{}

	for _, data_source := range responseData.Embedded.DataSources {
		customAttributes, _ := types.MapValueFrom(ctx, types.StringType, data_source.CustomAttributes)

		data.DataSources = append(data.DataSources, DataSourceModel{
			Id:                        types.StringValue(data_source.Id),
			Name:                      types.StringValue(data_source.Name),
			Description:               types.StringValue(data_source.Description),
			DataSourceToken:           types.StringValue(data_source.DataSourceToken),
			Adapter:                   types.StringValue(data_source.Adapter),
			CreatedAt:                 types.StringValue(data_source.CreatedAt),
			UpdatedAt:                 types.StringValue(data_source.UpdatedAt),
			HasExpensiveSchemaUpdates: types.BoolValue(data_source.HasExpensiveSchemaUpdates),
			Public:                    types.BoolValue(data_source.Public),
			Asleep:                    types.BoolValue(data_source.Asleep),
			Queryable:                 types.BoolValue(data_source.Queryable),
			SoftDeleted:               types.BoolValue(data_source.SoftDeleted),
			DisplayName:               types.StringValue(data_source.DisplayName),
			AccountId:                 types.StringValue(data_source.AccountId),
			AccountUsername:           types.StringValue(data_source.AccountUsername),
			OrganizationToken:         types.StringValue(data_source.OrganizationToken),
			OrganizationPlanCode:      types.StringValue(data_source.OrganizationPlanCode),
			Database:                  types.StringValue(data_source.Database),
			Host:                      types.StringValue(data_source.Host),
			Port:                      types.NumberValue(big.NewFloat(data_source.Port)),
			Ssl:                       types.BoolValue(data_source.Ssl),
			Username:                  types.StringValue(data_source.Username),
			Provider:                  types.StringValue(data_source.Provider),
			Vendor:                    types.StringValue(data_source.Vendor),
			Ldap:                      types.BoolValue(data_source.Ldap),
			Warehouse:                 types.StringValue(data_source.Warehouse),
			Bridged:                   types.BoolValue(data_source.Bridged),
			AdapterVersion:            types.StringValue(data_source.AdapterVersion),
			CustomAttributes:          customAttributes,
		})
	}

	tflog.Trace(ctx, "read a data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
