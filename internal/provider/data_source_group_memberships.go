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
var _ datasource.DataSource = &GroupMembershipsDataSource{}

func NewGroupMembershipsDataSource() datasource.DataSource {
	return &GroupMembershipsDataSource{}
}

// GroupMembershipsDataSource defines the data source implementation.
type GroupMembershipsDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

func (d *GroupMembershipsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_memberships"
}

func (d *GroupMembershipsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Group Memberships data source",

		Attributes: map[string]schema.Attribute{
			"state": schema.StringAttribute{
				MarkdownDescription: "State of the group",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the group",
				Computed:            true,
			},
			"group_token": schema.StringAttribute{
				MarkdownDescription: "Group configurable attribute",
				Required:            true,
			},
		},
	}
}

func (d *GroupMembershipsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GroupMembershipsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GroupResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Construct the URL using modeHost, workspaceId, and groupToken
	url := fmt.Sprintf("%s/api/%s/groups/%s/memberships", d.modeHost, d.workspaceId, data.GroupToken.ValueString())

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
		Name  string `json:"name"`
		State string `json:"state"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	// Assign the parsed values to the data model
	data.Name = types.StringValue(responseData.Name)
	data.State = types.StringValue(responseData.State)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
