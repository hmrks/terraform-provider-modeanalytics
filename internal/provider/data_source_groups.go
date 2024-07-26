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

var _ datasource.DataSource = &GroupsDataSource{}

func NewGroupsDataSource() datasource.DataSource {
	return &GroupsDataSource{}
}

type GroupsDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type GroupsDataSourceModel struct {
	Groups []GroupResourceModel `tfsdk:"groups"`
}

func (d *GroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

func (d *GroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Groups data source",

		Attributes: map[string]schema.Attribute{
			"groups": schema.ListAttribute{
				MarkdownDescription: "List of groups",
				Computed:            true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"group_token": types.StringType,
						"state":       types.StringType,
						"name":        types.StringType,
					},
				},
			},
		},
	}
}

func (d *GroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GroupsDataSourceModel

	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups", d.modeHost, d.workspaceId)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	httpResp, err := d.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list groups: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list groups: %s", httpResp.StatusCode))
		return
	}

	// Parse the response body
	var responseData struct {
		Embedded struct {
			Groups []struct {
				GroupToken string `json:"token"`
				Name       string `json:"name"`
				State      string `json:"state"`
			} `json:"groups"`
		} `json:"_embedded"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	data.Groups = []GroupResourceModel{}

	for _, group := range responseData.Embedded.Groups {
		data.Groups = append(data.Groups, GroupResourceModel{
			GroupToken: types.StringValue(group.GroupToken),
			Name:       types.StringValue(group.Name),
			State:      types.StringValue(group.State),
		})
	}

	tflog.Trace(ctx, "read a data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
