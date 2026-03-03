package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserResource struct {
	client *cephclient.Client
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Key             types.String `tfsdk:"key"`
	Caps            types.Map    `tfsdk:"caps"`
	RotationTrigger types.String `tfsdk:"rotation_trigger"`
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = UserSchema()
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Missing provider configuration")
		return
	}

	// Convert caps (if known)
	var caps map[string]string
	if !plan.Caps.IsNull() && !plan.Caps.IsUnknown() {
		resp.Diagnostics.Append(plan.Caps.ElementsAs(ctx, &caps, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	info, err := r.client.CreateUser(ctx, plan.Name.ValueString(), caps)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Name.ValueString())
	plan.Key = types.StringValue(info.Key)

	// Preserve null/unknown semantics
	if plan.Caps.IsNull() {
		plan.Caps = types.MapNull(types.StringType)
	} else if plan.Caps.IsUnknown() {
		// leave unknown
	} else {
		// concrete map — keep as-is
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Missing provider configuration")
		return
	}

	info, err := r.client.ReadUser(ctx, state.Name.ValueString())
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	// Load prior state to preserve null/unknown
	var prior UserModel
	_ = req.State.Get(ctx, &prior)

	// ID always equals name
	state.ID = types.StringValue(state.Name.ValueString())
	state.Key = types.StringValue(info.Key)

	// CAPS PRESERVATION LOGIC
	switch {
	case prior.Caps.IsNull():
		// User never set caps → keep null
		state.Caps = types.MapNull(types.StringType)

	case prior.Caps.IsUnknown():
		// Unknown stays unknown
		state.Caps = types.MapUnknown(types.StringType)

	default:
		// Concrete map → preserve exactly what user set
		state.Caps = prior.Caps
	}

	// ROTATION_TRIGGER PRESERVATION LOGIC
	// This is purely a Terraform-side trigger, so we always keep what the user had.
	state.RotationTrigger = prior.RotationTrigger

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserModel
	var state UserModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Missing provider configuration")
		return
	}

	// Start with the existing key; we may override it if we rotate.
	plan.Key = state.Key

	// Handle key rotation if rotation_trigger changed.
	// We treat any transition from null/unknown to a concrete value,
	// or a change between concrete values, as a rotation request.
	if !plan.RotationTrigger.IsUnknown() {
		rotate := false

		switch {
		case plan.RotationTrigger.IsNull():
			// User cleared the trigger → do not rotate.
			rotate = false

		case state.RotationTrigger.IsUnknown(), state.RotationTrigger.IsNull():
			// Previously unset, now set → rotate.
			rotate = true

		default:
			if plan.RotationTrigger.ValueString() != state.RotationTrigger.ValueString() {
				rotate = true
			}
		}

		if rotate {
			fmt.Println("[DEBUG] terraform: rotation_trigger changed, rotating key for", plan.Name.ValueString())

			info, err := r.client.RotateUserKey(ctx, plan.Name.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("Rotate Error", err.Error())
				return
			}

			plan.Key = types.StringValue(info.Key)
		}
	}

	// Convert caps if known and update them (caps only, no key changes here).
	var caps map[string]string
	if !plan.Caps.IsNull() && !plan.Caps.IsUnknown() {
		resp.Diagnostics.Append(plan.Caps.ElementsAs(ctx, &caps, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.client.UpdateUser(ctx, plan.Name.ValueString(), caps); err != nil {
			resp.Diagnostics.AddError("Update Error", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(plan.Name.ValueString())

	// Preserve null/unknown semantics for caps
	if plan.Caps.IsNull() {
		plan.Caps = types.MapNull(types.StringType)
	} else if plan.Caps.IsUnknown() {
		// leave unknown
	} else {
		// concrete map — keep as-is
	}

	// rotation_trigger is purely config; just keep whatever is in plan.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	err := r.client.DeleteUser(ctx, state.Name.ValueString())
	if err != nil && !errors.Is(err, cephclient.ErrNotFound) {
		resp.Diagnostics.AddError("Delete Error", err.Error())
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 1 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected: username (e.g. client.backup)")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[0])...)
}
