package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &WorkspaceMembershipsDataSource{}

func NewWorkspaceMembershipsDataSource() datasource.DataSource {
	return &WorkspaceMembershipsDataSource{}
}

type WorkspaceMembershipsDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type WorkspaceMemberModel struct {
	Admin          types.Bool   `tfsdk:"admin"`
	State          types.String `tfsdk:"state"`
	MemberUsername types.String `tfsdk:"member_username"`
	MemberToken    types.String `tfsdk:"member_token"`
	ActivatedAt    types.String `tfsdk:"activated_at"`
}

type WorkspaceMembershipsDataSourceModel struct {
	Memberships []WorkspaceMemberModel `tfsdk:"memberships"`
}

func (d *WorkspaceMembershipsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_memberships"
}

func (d *WorkspaceMembershipsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Workspace memberships data source",

		Attributes: map[string]schema.Attribute{
			"memberships": schema.ListAttribute{
				MarkdownDescription: "List of workspace memberships",
				Computed:            true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"admin":           types.BoolType,
						"state":           types.StringType,
						"member_username": types.StringType,
						"member_token":    types.StringType,
						"activated_at":    types.StringType,
					},
				},
			},
		},
	}
}

func (d *WorkspaceMembershipsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkspaceMembershipsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceMembershipsDataSourceModel

	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/memberships", d.modeHost, d.workspaceId)

	httpReq, err := HttpRetry(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %s", err))
		return
	}

	httpResp, err := d.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list memberships: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list memberships: %d", httpResp.StatusCode))
		return
	}

	var responseData struct {
		Embedded struct {
			Memberships []struct {
				Admin          bool   `json:"admin"`
				State          string `json:"state"`
				MemberUsername string `json:"member_username"`
				MemberToken    string `json:"member_token"`
				ActivatedAt    string `json:"activated_at"`
			} `json:"memberships"`
		} `json:"_embedded"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	data.Memberships = []WorkspaceMemberModel{}

	for _, membership := range responseData.Embedded.Memberships {
		data.Memberships = append(data.Memberships, WorkspaceMemberModel{
			Admin:          types.BoolValue(membership.Admin),
			State:          types.StringValue(membership.State),
			MemberUsername: types.StringValue(membership.MemberUsername),
			MemberToken:    types.StringValue(membership.MemberToken),
			ActivatedAt:    types.StringValue(membership.ActivatedAt),
		})
	}

	tflog.Trace(ctx, "read a data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
