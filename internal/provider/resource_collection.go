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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CollectionResource{}

// NewCollectionResource returns a new instance of CollectionResource.
func NewCollectionResource() resource.Resource {
	return &CollectionResource{}
}

// CollectionResource defines the resource implementation.
type CollectionResource struct {
	client      *http.Client
	modeHost    string
	workspaceId string
}

type Collection struct {
	CollectionType     string `json:"space_type"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Restricted         bool   `json:"restricted"`
	FreeDefault        bool   `json:"free_default"`
	Viewable           bool   `json:"viewable?"`
	DefaultAccessLevel string `json:"default_access_level"`
}

type CollectionPayload struct {
	Collection Collection `json:"space"`
}

// Metadata sets the resource type name.
func (r *CollectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collection"
}

// Schema defines the resource schema.
func (r *CollectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"collection_token": schema.StringAttribute{
				MarkdownDescription: "State of the collection",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Name of the collection",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "State of the collection",
				Computed:            true,
			},
			"collection_type": schema.StringAttribute{
				MarkdownDescription: "Collection configurable attribute",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("custom"),
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the collection",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the collection",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"restricted": schema.BoolAttribute{
				MarkdownDescription: "Restricted attribute of the collection",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"free_default": schema.BoolAttribute{
				MarkdownDescription: "Free default attribute of the collection",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"viewable": schema.BoolAttribute{
				MarkdownDescription: "Viewable attribute of the collection",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"default_access_level": schema.StringAttribute{
				MarkdownDescription: "Default access level attribute of the collection",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("restricted"),
			},
		},
	}
}

// Configure sets the resource client.
func (r *CollectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *CollectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CollectionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces", r.modeHost, r.workspaceId)

	payload := CollectionPayload{
		Collection: Collection{
			CollectionType:     plan.CollectionType.ValueString(),
			Name:               plan.Name.ValueString(),
			Description:        plan.Description.ValueString(),
			Restricted:         plan.Restricted.ValueBool(),
			FreeDefault:        plan.FreeDefault.ValueBool(),
			Viewable:           plan.Viewable.ValueBool(),
			DefaultAccessLevel: plan.DefaultAccessLevel.ValueString(),
		},
	}
	if plan.DefaultAccessLevel.ValueString() == "restricted" {
		payload.Collection.DefaultAccessLevel = "none"
	}

	jsonBody, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("One Unable to create collection, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)

	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Two Unable to create collection, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		Id                 string `json:"id"`
		Name               string `json:"name"`
		State              string `json:"state"`
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

	plan.CollectionToken = types.StringValue(responseData.CollectionToken)
	plan.State = types.StringValue(responseData.State)
	plan.Id = types.StringValue(responseData.Id)
	plan.Restricted = types.BoolValue(responseData.Restricted)
	plan.FreeDefault = types.BoolValue(responseData.FreeDefault)
	plan.Viewable = types.BoolValue(responseData.Viewable)
	plan.DefaultAccessLevel = types.StringValue(responseData.DefaultAccessLevel)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read handles reading the resource.
func (r *CollectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CollectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s", r.modeHost, r.workspaceId, state.CollectionToken.ValueString())
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		Id                 string `json:"id"`
		Name               string `json:"name"`
		State              string `json:"state"`
		CollectionType     string `json:"space_type"`
		CollectionToken    string `json:"token"`
		Description        string `json:"description"`
		Restricted         bool   `json:"restricted"`
		FreeDefault        bool   `json:"free_default"`
		Viewable           bool   `json:"viewable?"`
		DefaultAccessLevel string `json:"default_access_level"`
	}

	if httpResp.StatusCode == http.StatusOK {
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
		state.CollectionType = types.StringValue(responseData.CollectionType)
		state.Description = types.StringValue(responseData.Description)
		state.Restricted = types.BoolValue(responseData.Restricted)
		state.FreeDefault = types.BoolValue(responseData.FreeDefault)
		state.Viewable = types.BoolValue(responseData.Viewable)
		state.DefaultAccessLevel = types.StringValue(responseData.DefaultAccessLevel)

		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		// This is horrible and should be reworked
	} else if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
	} else if httpResp.StatusCode == http.StatusForbidden {
		// There is a bug where a GET request on a freshly deleted collection returns 403 instead of 404.
		// So as a workaround, we list all collections. If we have the correct access rights to do so,
		// we assume everything is alright.
		url := fmt.Sprintf("%s/api/%s/spaces?filter=all", r.modeHost, r.workspaceId)
		httpReq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection, got error: %s", err))
			return
		}

		httpResp, err := HttpRetry(r.client, httpReq)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection, got error: %s", err))
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode == http.StatusOK {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read collection, got error: %s", err))
			return
		}
	} else {
		resp.Diagnostics.AddError("API response error", fmt.Sprintf("Received non-200 response status: %d", httpResp.StatusCode))
	}
}

// Update handles updating the resource.
func (r *CollectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CollectionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s", r.modeHost, r.workspaceId, plan.CollectionToken.ValueString())
	payload := CollectionPayload{
		Collection: Collection{
			CollectionType:     plan.CollectionType.ValueString(),
			Name:               plan.Name.ValueString(),
			Description:        plan.Description.ValueString(),
			Restricted:         plan.Restricted.ValueBool(),
			FreeDefault:        plan.FreeDefault.ValueBool(),
			Viewable:           plan.Viewable.ValueBool(),
			DefaultAccessLevel: plan.DefaultAccessLevel.ValueString(),
		},
	}
	jsonBody, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update collection, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update collection, got error: %s", url))
		return
	}
	defer httpResp.Body.Close()

	var responseData struct {
		Id                 string `json:"id"`
		Name               string `json:"name"`
		State              string `json:"state"`
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

	plan.Name = types.StringValue(responseData.Name)
	plan.State = types.StringValue(responseData.State)
	plan.CollectionType = types.StringValue(responseData.CollectionType)
	plan.Description = types.StringValue(responseData.Description)
	plan.Restricted = types.BoolValue(responseData.Restricted)
	plan.FreeDefault = types.BoolValue(responseData.FreeDefault)
	plan.Viewable = types.BoolValue(responseData.Viewable)
	plan.DefaultAccessLevel = types.StringValue(responseData.DefaultAccessLevel)

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete handles deleting the resource.
func (r *CollectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CollectionModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/api/%s/spaces/%s", r.modeHost, r.workspaceId, state.CollectionToken.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete collection, got error: %s", err))
		return
	}

	httpResp, err := HttpRetry(r.client, httpReq)
	if err != nil || httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete collection, got error: %v", httpResp))
		return
	}
	defer httpResp.Body.Close()

	// Verify deletion of the resource
	deletionErr := CheckDeletion(url, r.client)
	if deletionErr != nil {
		resp.Diagnostics.AddError("Collection Deletion Error", fmt.Sprintf("Failed to verify deletion: %s", deletionErr))
		return
	}

	// Remove the resource from the state
	resp.State.RemoveResource(ctx)
}

func (r *CollectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("collection_token"), req.ID)...)
}
