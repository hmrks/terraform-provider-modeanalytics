package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &CollectionDataSource{}

func NewCollectionDataSource() datasource.DataSource {
	return &CollectionDataSource{}
}

// CollectionDataSource defines the data source implementation.
type CollectionDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type CollectionModel struct {
	Name               types.String `tfsdk:"name"`
	State              types.String `tfsdk:"state"`
	CollectionToken    types.String `tfsdk:"collection_token"`
	CollectionType     types.String `tfsdk:"collection_type"`
	Id                 types.String `tfsdk:"id"`
	Description        types.String `tfsdk:"description"`
	Restricted         types.Bool   `tfsdk:"restricted"`
	FreeDefault        types.Bool   `tfsdk:"free_default"`
	Viewable           types.Bool   `tfsdk:"viewable"`
	DefaultAccessLevel types.String `tfsdk:"default_access_level"`
}

func (d *CollectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collection"
}

func (d *CollectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Collection data source",
		Attributes: map[string]schema.Attribute{
			"collection_token": schema.StringAttribute{
				MarkdownDescription: "State of the collection",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Name of the collection",
				Computed:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "State of the collection",
				Computed:            true,
			},
			"collection_type": schema.StringAttribute{
				MarkdownDescription: "Collection configurable attribute",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the collection",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the collection",
				Computed:            true,
			},
			"restricted": schema.BoolAttribute{
				MarkdownDescription: "Restricted attribute of the collection",
				Computed:            true,
			},
			"free_default": schema.BoolAttribute{
				MarkdownDescription: "Free default attribute of the collection",
				Computed:            true,
			},
			"viewable": schema.BoolAttribute{
				MarkdownDescription: "Viewable attribute of the collection",
				Computed:            true,
			},
			"default_access_level": schema.StringAttribute{
				MarkdownDescription: "Default access level attribute of the collection",
				Computed:            true,
			},
		},
	}
}

func (d *CollectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CollectionModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Construct the URL using modeHost, workspaceId, and groupToken
	url := fmt.Sprintf("%s/api/%s/spaces/%s", d.modeHost, d.workspaceId, data.CollectionToken.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %s", err))
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
		Name               string `json:"name"`
		State              string `json:"state"`
		Id                 string `json:"id"`
		CollectionType     string `json:"space_type"`
		CollectionToken    string `json:"token"`
		Description        string `json:"description"`
		Restricted         bool   `json:"restricted"`
		FreeDefault        bool   `json:"free_default"`
		Viewable           bool   `json:"viewable?"`
		DefaultAccessLevel string `json:"default_access_level"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	// Assign the parsed values to the data model
	data.Name = types.StringValue(responseData.Name)
	data.State = types.StringValue(responseData.State)
	data.Id = types.StringValue(responseData.Id)
	data.CollectionType = types.StringValue(responseData.CollectionType)
	data.CollectionToken = types.StringValue(responseData.CollectionToken)
	data.Description = types.StringValue(responseData.Description)
	data.Restricted = types.BoolValue(responseData.Restricted)
	data.FreeDefault = types.BoolValue(responseData.FreeDefault)
	data.Viewable = types.BoolValue(responseData.Viewable)
	data.DefaultAccessLevel = types.StringValue(responseData.DefaultAccessLevel)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
