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

var _ datasource.DataSource = &CollectionsDataSource{}

func NewCollectionsDataSource() datasource.DataSource {
	return &CollectionsDataSource{}
}

type CollectionsDataSource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type CollectionsDataSourceModel struct {
	Collections []CollectionModel `tfsdk:"collections"`
}

func (d *CollectionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collections"
}

func (d *CollectionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Collections data source",

		Attributes: map[string]schema.Attribute{
			"collections": schema.ListAttribute{
				MarkdownDescription: "List of collections",
				Computed:            true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"id":                   types.NumberType,
						"name":                 types.StringType,
						"state":                types.StringType,
						"collection_token":     types.StringType,
						"collection_type":      types.StringType,
						"description":          types.StringType,
						"restricted":           types.BoolType,
						"free_default":         types.BoolType,
						"viewable":             types.BoolType,
						"default_access_level": types.StringType,
					},
				},
			},
		},
	}
}

func (d *CollectionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CollectionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CollectionsDataSourceModel

	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces?filter=all", d.modeHost, d.workspaceId)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %s", err))
		return
	}

	httpResp, err := HttpRetry(d.client, httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list collections: %d", httpResp.StatusCode))
		return
	}

	// Parse the response body
	var responseData struct {
		Embedded struct {
			Collections []struct {
				Id                 float64 `json:"id"`
				Name               string  `json:"name"`
				State              string  `json:"state"`
				CollectionToken    string  `json:"token"`
				CollectionType     string  `json:"space_type"`
				Description        string  `json:"description"`
				Restricted         bool    `json:"restricted"`
				FreeDefault        bool    `json:"free_default"`
				Viewable           bool    `json:"viewable?"`
				DefaultAccessLevel string  `json:"default_access_level"`
			} `json:"spaces"`
		} `json:"_embedded"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	data.Collections = []CollectionModel{}

	for _, collection := range responseData.Embedded.Collections {
		data.Collections = append(data.Collections, CollectionModel{
			Id:                 types.NumberValue(big.NewFloat(collection.Id)),
			Name:               types.StringValue(collection.Name),
			State:              types.StringValue(collection.State),
			CollectionToken:    types.StringValue(collection.CollectionToken),
			CollectionType:     types.StringValue(collection.CollectionType),
			Description:        types.StringValue(collection.Description),
			Restricted:         types.BoolValue(collection.Restricted),
			FreeDefault:        types.BoolValue(collection.FreeDefault),
			Viewable:           types.BoolValue(collection.Viewable),
			DefaultAccessLevel: types.StringValue(collection.DefaultAccessLevel),
		})
	}

	tflog.Trace(ctx, "read a data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
