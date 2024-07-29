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
var _ resource.Resource = &CollectionPermissionResource{}

// NewCollectionPermissionResource returns a new instance of CollectionPermissionResource.
func NewCollectionPermissionResource() resource.Resource {
	return &CollectionPermissionResource{}
}

// CollectionPermissionResource defines the resource implementation.
type CollectionPermissionResource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

// CollectionPermissionResourceModel describes the resource data model.
type CollectionPermissionResourceModel struct {
	CollectionToken types.String `tfsdk:"collection_token"`
	Action          types.String `tfsdk:"action"`
	AccessorToken   types.String `tfsdk:"accessor_token"`
	AccessorType    types.String `tfsdk:"accessor_type"`
	PermissionToken types.String `tfsdk:"permission_token"`
}

type CollectionPermission struct {
	Action        string `json:"action"`
	AccessorType  string `json:"accessor_type"`
	AccessorToken string `json:"accessor_token"`
}

type CollectionPermissionPayload struct {
	Permission CollectionPermission `json:"permission"`
}

type UpdateCollectionPermission struct {
	Action string `json:"action"`
}

type CollectionPermissionUpdatePayload struct {
	Permission UpdateCollectionPermission `json:"permission"`
}

// Metadata sets the resource type name.
func (r *CollectionPermissionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collection_permission"
}

// Schema defines the resource schema.
func (r *CollectionPermissionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"collection_token": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"action": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"view", "edit"}...),
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
func (r *CollectionPermissionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *CollectionPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CollectionPermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s/permissions", r.modeHost, r.workspaceId, plan.CollectionToken.ValueString())

	payload := CollectionPermissionPayload{
		Permission: CollectionPermission{
			Action:        plan.Action.ValueString(),
			AccessorType:  plan.AccessorType.ValueString(),
			AccessorToken: plan.AccessorToken.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create collection permission, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create collection permission, got error: %v", httpResp))
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
func (r *CollectionPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CollectionPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s/permissions/%s", r.modeHost, r.workspaceId, state.CollectionToken.ValueString(), state.PermissionToken.ValueString())
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection permission, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection permission, got error: %s", err))
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
	} else {
		resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", httpResp.StatusCode))
	}
}

// Update handles updating the resource.
func (r *CollectionPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CollectionPermissionResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s/permissions/%s", r.modeHost, r.workspaceId, plan.CollectionToken.ValueString(), plan.PermissionToken.ValueString())
	payload := CollectionPermissionUpdatePayload{
		Permission: UpdateCollectionPermission{
			Action: plan.Action.ValueString(),
		},
	}

	jsonBody, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update collection permission, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update collection permission, got error: %s", url))
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
func (r *CollectionPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CollectionPermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s/permissions/%s", r.modeHost, r.workspaceId, state.CollectionToken.ValueString(), state.PermissionToken.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete collection permission, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete collection permission, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	// Verify deletion of the resource
	deletionErr := CheckDeletion(url, r.client)
	if deletionErr != nil {
		resp.Diagnostics.AddError("Collection Permission Deletion Error", fmt.Sprintf("Failed to verify deletion: %s", deletionErr))
		return
	}

	// Remove the resource from the state
	resp.State.RemoveResource(ctx)
}

func (r *CollectionPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permission_token"), req.ID)...)
}
