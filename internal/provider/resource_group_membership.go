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
var _ resource.Resource = &GroupMembershipResource{}

// NewGroupMembershipResource returns a new instance of GroupMembershipResource.
func NewGroupMembershipResource() resource.Resource {
	return &GroupMembershipResource{}
}

// GroupMembershipResource defines the resource implementation.
type GroupMembershipResource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

// GroupMembershipResourceModel describes the resource data model.
type GroupMembershipResourceModel struct {
	GroupToken      types.String `tfsdk:"group_token"`
	MemberToken     types.String `tfsdk:"member_token"`
	MembershipToken types.String `tfsdk:"membership_token"`
}

type Membership struct {
	MemberToken string `json:"member_token"`
}

type GroupMembershipPayload struct {
	Membership Membership `json:"membership"`
}

// Metadata sets the resource type name.
func (r *GroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

// Schema defines the resource schema.
func (r *GroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"group_token": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"member_token": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"membership_token": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure sets the resource client.
func (r *GroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *GroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GroupMembershipResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s/memberships", r.modeHost, r.workspaceId, plan.GroupToken.ValueString())

	payload := GroupMembershipPayload{
		Membership: Membership{
			MemberToken: plan.MemberToken.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)

	httpReq, err := HttpRetry(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("One Unable to create group membership, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Two Unable to create group membership, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		MembershipToken string `json:"token"`
	}

	err = json.NewDecoder(httpResp.Body).Decode(&responseData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
		return
	}

	plan.MembershipToken = types.StringValue(responseData.MembershipToken)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read handles reading the resource.
func (r *GroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GroupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s/memberships/%s", r.modeHost, r.workspaceId, state.GroupToken.ValueString(), state.MembershipToken.ValueString())
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group membership, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group membership, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		MembershipToken string `json:"token"`
		MemberToken     string `json:"member_token"`
	}

	if httpResp.StatusCode == http.StatusOK {
		err = json.NewDecoder(httpResp.Body).Decode(&responseData)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Error parsing response: %s", err))
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	} else if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
	} else {
		resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", httpResp.StatusCode))
	}
}

// Update handles updating the resource.
func (r *GroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GroupMembershipResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete handles deleting the resource.
func (r *GroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GroupMembershipResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/groups/%s/memberships/%s", r.modeHost, r.workspaceId, state.GroupToken.ValueString(), state.MembershipToken.ValueString())
	httpReq, err := HttpRetry(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete group membership, got error: %s", err))
		return
	}

	httpResp, err := r.client.Do(httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete group membership, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	// Verify deletion of the resource
	deletionErr := CheckDeletion(url, r.client)
	if deletionErr != nil {
		resp.Diagnostics.AddError("Group Membership Deletion Error", fmt.Sprintf("Failed to verify deletion: %s", deletionErr))
		return
	}

	// Remove the resource from the state
	resp.State.RemoveResource(ctx)
}

func (r *GroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("membership_token"), req.ID)...)
}
