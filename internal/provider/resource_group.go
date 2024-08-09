package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &GroupResource{}

// NewGroupResource returns a new instance of GroupResource.
func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

// GroupResource defines the resource implementation.
type GroupResource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

// GroupResourceModel describes the resource data model.
type GroupResourceModel struct {
	GroupToken types.String `tfsdk:"group_token"`
	Name       types.String `tfsdk:"name"`
	State      types.String `tfsdk:"state"`
}

type UserGroup struct {
	Name string `json:"name"`
}

type Payload struct {
	UserGroup UserGroup `json:"user_group"`
}

// Metadata sets the resource type name.
func (r *GroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

// Schema defines the resource schema.
func (r *GroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"group_token": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure sets the resource client.
func (r *GroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups", r.modeHost, r.workspaceId)

	payload := Payload{
		UserGroup: UserGroup{
			Name: plan.Name.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)

	httpReq, err := HttpRetry(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("One Unable to create group, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Two Unable to create group, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		GroupToken string `json:"token"`
		Name       string `json:"name"`
		State      string `json:"state"`
	}
	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	plan.GroupToken = types.StringValue(responseData.GroupToken)
	plan.State = types.StringValue(responseData.State)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read handles reading the resource.
func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s", r.modeHost, r.workspaceId, state.GroupToken.ValueString())
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusOK {
		var responseData struct {
			State string `json:"state"`
			Name  string `json:"name"`
		}
		err = json.NewDecoder(httpResp.Body).Decode(&responseData)

		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
			return
		}
		if responseData.State == "soft_deleted" {
			resp.State.RemoveResource(ctx)
			return
		}
		state.State = types.StringValue(responseData.State)
		state.Name = types.StringValue(responseData.Name)

		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	} else if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
	} else {
		resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", httpResp.StatusCode))
	}
}

// Update handles updating the resource.
func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s", r.modeHost, r.workspaceId, plan.GroupToken.ValueString())
	payload := Payload{
		UserGroup: UserGroup{
			Name: plan.Name.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)
	httpReq, err := HttpRetry(ctx, http.MethodPatch, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update group, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update group, got error: %s", url))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	plan.State = types.StringValue(responseData.State)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete handles deleting the resource.
func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s", r.modeHost, r.workspaceId, state.GroupToken.ValueString())
	httpReq, err := HttpRetry(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete group, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete group, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	// Verify deletion of the resource
	deletionErr := CheckDeletion(url, r.client)
	if deletionErr != nil {
		resp.Diagnostics.AddError("Group Deletion Error. If the name of the group matches one that was already deleted, its name needs to be changed before it can be deleted (API limitation)", fmt.Sprintf("Failed to verify deletion: %s", deletionErr))
		return
	}

	// Remove the resource from the state
	resp.State.RemoveResource(ctx)
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_token"), req.ID)...)
}
