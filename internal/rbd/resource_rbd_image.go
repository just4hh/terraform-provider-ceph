package rbd

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

type RBDImageResource struct {
	client *cephclient.Client
}

func NewRBDImageResource() resource.Resource {
	return &RBDImageResource{}
}

type RBDImageModel struct {
	ID       types.String `tfsdk:"id"`
	Pool     types.String `tfsdk:"pool"`
	Name     types.String `tfsdk:"name"`
	Size     types.Int64  `tfsdk:"size"`
	Features types.List   `tfsdk:"features"`
}

func (r *RBDImageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbd_image"
}

func (r *RBDImageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = RBDImageSchema()
}

func (r *RBDImageResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *RBDImageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RBDImageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured. Ensure the provider block is present and valid.",
		)
		return
	}

	if err := r.client.CreateImage(ctx, plan.Pool.ValueString(), plan.Name.ValueString(), uint64(plan.Size.ValueInt64())); err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	// ID is pool/name
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.Pool.ValueString(), plan.Name.ValueString()))

	// Preserve features semantics: if the plan left features null, keep null in state.
	// If the plan explicitly provided a list (even empty), keep that list.
	// If features are unknown, leave them unknown (do not coerce).
	// if plan.Features.IsNull() {
	// 	plan.Features = types.ListNull(types.StringType)
	// } else if plan.Features.IsUnknown() {
	// 	// leave as-is (unknown) so Terraform can resolve it later
	// } else {
	// 	// plan.Features is a concrete list — keep it unchanged
	// }

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RBDImageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RBDImageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If provider wasn't configured correctly
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured. Ensure the provider block is present and valid.",
		)
		return
	}

	info, err := r.client.GetImage(ctx, state.Pool.ValueString(), state.Name.ValueString())
	if err != nil {
		// Image no longer exists → remove from state
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	// after fetching info from Ceph and before setting state:
	var prior RBDImageModel
	_ = req.State.Get(ctx, &prior) // ignore diagnostics here; use prior if present
	state.Features = prior.Features
	// if prior.Features.IsNull() {
	// 	// keep null
	// 	state.Features = types.ListNull(types.StringType)
	// } else {
	// 	// if provider cannot discover features, keep prior concrete list
	// 	// (or set to ListNull if prior was null)
	// 	state.Features = prior.Features
	// }

	// Rebuild full state
	state.ID = types.StringValue(fmt.Sprintf("%s/%s", state.Pool.ValueString(), state.Name.ValueString()))
	state.Size = types.Int64Value(int64(info.Size))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RBDImageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RBDImageModel
	var state RBDImageModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured. Ensure the provider block is present and valid.",
		)
		return
	}

	// Resize (only allow growing)
	if plan.Size.ValueInt64() < state.Size.ValueInt64() {
		resp.Diagnostics.AddError("Invalid Update", "Shrinking RBD images is not supported")
		return
	}

	if plan.Size.ValueInt64() > state.Size.ValueInt64() {
		if err := r.client.ResizeImage(ctx, plan.Pool.ValueString(), plan.Name.ValueString(), uint64(plan.Size.ValueInt64())); err != nil {
			resp.Diagnostics.AddError("Resize Error", err.Error())
			return
		}
	}

	// Rename
	if plan.Name.ValueString() != state.Name.ValueString() {
		if err := r.client.RenameImage(ctx, plan.Pool.ValueString(), state.Name.ValueString(), plan.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("Rename Error", err.Error())
			return
		}
	}

	// Always set ID to a known value after apply
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.Pool.ValueString(), plan.Name.ValueString()))

	// Preserve features semantics: if the plan left features null, keep null in state.
	// If the plan explicitly provided a list (even empty), keep that list.
	// If features are unknown, leave them unknown (do not coerce).
	// if plan.Features.IsNull() {
	// 	plan.Features = types.ListNull(types.StringType)
	// } else if plan.Features.IsUnknown() {
	// 	// leave as-is (unknown) so Terraform can resolve it later
	// } else {
	// 	// plan.Features is a concrete list — keep it unchanged
	// }

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RBDImageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RBDImageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		// Treat as already deleted
		return
	}

	if err := r.client.DeleteImage(ctx, state.Pool.ValueString(), state.Name.ValueString()); err != nil {
		// If already gone, treat as success
		if errors.Is(err, cephclient.ErrNotFound) {
			return
		}
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *RBDImageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: pool/name")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pool"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}
