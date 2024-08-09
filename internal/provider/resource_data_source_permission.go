package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DataSourcePermissionResource{}

// NewDataSourcePermissionResource returns a new instance of DataSourcePermissionResource.
func NewDataSourcePermissionResource() resource.Resource {
	return &DataSourcePermissionResource{}
}

// DataSourcePermissionResource defines the resource implementation.
type DataSourcePermissionResource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

// DataSourcePermissionResourceModel describes the resource data model.
type DataSourcePermissionResourceModel struct {
	DataSourceToken types.String `tfsdk:"data_source_token"`
	Action          types.String `tfsdk:"action"`
	AccessorToken   types.String `tfsdk:"accessor_token"`
	AccessorType    types.String `tfsdk:"accessor_type"`
	PermissionToken types.String `tfsdk:"permission_token"`
}

type Permission struct {
	Action        string `json:"action"`
	AccessorType  string `json:"accessor_type"`
	AccessorToken string `json:"accessor_token"`
}

type DataSourcePermissionPayload struct {
	Permission Permission `json:"permission"`
}

type UpdatePermission struct {
	Action string `json:"action"`
}

type DataSourcePermissionUpdatePayload struct {
	Permission UpdatePermission `json:"permission"`
}

// Metadata sets the resource type name.
func (r *DataSourcePermissionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_source_permission"
}

// Schema defines the resource schema.
func (r *DataSourcePermissionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"data_source_token": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"action": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"manage", "view", "query"}...),
				},
			},
			"accessor_token": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"accessor_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"Account", "UserGroup"}...),
				},
				Default: stringdefault.StaticString("Account"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission_token": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure sets the resource client.
func (r *DataSourcePermissionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected struct with *http.Client, ModeHost, and WorkspaceId, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = config.Client
	r.modeHost = config.ModeHost
	r.workspaceId = config.WorkspaceId
}

// Create handles the creation of the resource.
func (r *DataSourcePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DataSourcePermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/data_sources/%s/permissions", r.modeHost, r.workspaceId, plan.DataSourceToken.ValueString())

	payload := DataSourcePermissionPayload{
		Permission: Permission{
			Action:        plan.Action.ValueString(),
			AccessorType:  plan.AccessorType.ValueString(),
			AccessorToken: plan.AccessorToken.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("One Unable to create data source permission, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Two Unable to create data source permission, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		PermissionToken string `json:"token"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	plan.PermissionToken = types.StringValue(responseData.PermissionToken)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read handles reading the resource.
func (r *DataSourcePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DataSourcePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/data_sources/%s/permissions/%s", r.modeHost, r.workspaceId, state.DataSourceToken.ValueString(), state.PermissionToken.ValueString())
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data source permission, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data source permission, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		PermissionToken string `json:"token"`
		Action          string `json:"action"`
	}

	if httpResp.StatusCode == http.StatusOK {
		err = json.NewDecoder(httpResp.Body).Decode(&responseData)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
			return
		}

		state.Action = types.StringValue(responseData.Action)

		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	} else if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
	} else if httpResp.StatusCode == http.StatusInternalServerError {

		list_url := fmt.Sprintf("%s/api/%s/data_sources/%s/permissions", r.modeHost, r.workspaceId, state.DataSourceToken.ValueString())

		listHttpReq, err := http.NewRequest("GET", list_url, nil)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data source permission, got error: %s", err))
			return
		}
		listHttpResp, err := HttpRetry(r.client, listHttpReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data source permission, got error: %s", err))
			return
		}
		defer listHttpResp.Body.Close()

		var listResponseData struct {
			Embedded struct {
				Entitlements []struct {
					PermissionToken string `json:"token"`
					Action          string `json:"action"`
				} `json:"data_source_entitlements"`
			} `json:"_embedded"`
		}

		if listHttpResp.StatusCode == http.StatusOK {
			err = json.NewDecoder(listHttpResp.Body).Decode(&listResponseData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
				return
			}

			var found bool

			for _, entitlement := range listResponseData.Embedded.Entitlements {
				if entitlement.PermissionToken == state.PermissionToken.ValueString() {
					state.Action = types.StringValue(entitlement.Action)
					resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
					found = true
					break
				}
			}

			if !found {
				resp.State.RemoveResource(ctx)
			}
		} else {
			resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", listHttpResp.StatusCode))
		}
	} else {
		resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", httpResp.StatusCode))
	}
}

// Update handles updating the resource.
func (r *DataSourcePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DataSourcePermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/data_sources/%s/permissions/%s", r.modeHost, r.workspaceId, plan.DataSourceToken.ValueString(), plan.PermissionToken.ValueString())
	payload := DataSourcePermissionUpdatePayload{
		Permission: UpdatePermission{
			Action: plan.Action.ValueString(),
		},
	}

	jsonBody, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update data source permission, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update data source permission, got error: %s", url))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		PermissionToken string `json:"token"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	plan.PermissionToken = types.StringValue(responseData.PermissionToken)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete handles deleting the resource.
func (r *DataSourcePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DataSourcePermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/data_sources/%s/permissions/%s", r.modeHost, r.workspaceId, state.DataSourceToken.ValueString(), state.PermissionToken.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete data source permission, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete data source permission, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	// Verify deletion of the resource
	deletionErr := CheckDeletion(url, r.client)
	if deletionErr != nil {
		resp.Diagnostics.AddError("Data Source Permission Deletion Error", fmt.Sprintf("Failed to verify deletion: %s", deletionErr))
		return
	}

	// Remove the resource from the state
	resp.State.RemoveResource(ctx)
}

func (r *DataSourcePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permission_token"), req.ID)...)
}
