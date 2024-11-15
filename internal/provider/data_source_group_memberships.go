package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"io"
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
		MarkdownDescription: "Data source for retrieving member tokens of a group",

		Attributes: map[string]schema.Attribute{
			"group_token": schema.StringAttribute{
				MarkdownDescription: "The token identifying the group.",
				Required:            true,
			},
			"member_tokens": schema.ListAttribute{
				MarkdownDescription: "A list of member tokens in the group.",
				Computed:            true,
				ElementType:         types.StringType,
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
	// Define a struct matching the schema with both group_token and member_tokens
	var data struct {
		GroupToken   types.String `tfsdk:"group_token"`
		MemberTokens types.List   `tfsdk:"member_tokens"`
	}

	// Only retrieve group_token from config; member_tokens will be computed
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Construct the URL using modeHost, workspaceId, and groupToken
	url := fmt.Sprintf("%s/api/%s/groups/%s/memberships", d.modeHost, d.workspaceId, data.GroupToken.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := d.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch memberships: %s", err))
		return
	}
	defer httpResp.Body.Close()

	// Read and log the entire response body for debugging
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	// Parse the response JSON
	var membershipsResponse struct {
		Embedded struct {
			GroupMemberships []struct {
				MemberToken string `json:"member_token"`
			} `json:"group_memberships"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(bodyBytes, &membershipsResponse); err != nil {
		resp.Diagnostics.AddError("Decode Error", fmt.Sprintf("Error decoding response: %s", err))
		return
	}

	// Convert member tokens to a list of terraform values
	memberTokens := make([]attr.Value, len(membershipsResponse.Embedded.GroupMemberships))
	for i, membership := range membershipsResponse.Embedded.GroupMemberships {
		memberTokens[i] = types.StringValue(membership.MemberToken)
	}

	// Convert to ListValue
	memberTokensList, diags := types.ListValue(types.StringType, memberTokens)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set the computed value
	data.MemberTokens = memberTokensList

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Set the state
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
