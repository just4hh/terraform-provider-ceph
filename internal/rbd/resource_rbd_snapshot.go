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

type snapshotModel struct {
	ID          types.String `tfsdk:"id"`
	Pool        types.String `tfsdk:"pool"`
	Image       types.String `tfsdk:"image"`
	Name        types.String `tfsdk:"name"`
	Protected   types.Bool   `tfsdk:"protected"`
	CreatedAt   types.String `tfsdk:"created_at"`
	ForceDelete types.Bool   `tfsdk:"force_delete"`
}

type rbdSnapshotResource struct {
	client *cephclient.Client
}

func NewRBDSnapshotResource() resource.Resource {
	return &rbdSnapshotResource{}
}

func (r *rbdSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbd_snapshot"
}

func (r *rbdSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = RBDSnapshotSchema()
}

func (r *rbdSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*cephclient.Client)
	}
}

func (r *rbdSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan snapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool := plan.Pool.ValueString()
	image := plan.Image.ValueString()
	name := plan.Name.ValueString()

	// Create snapshot
	if err := r.client.CreateSnapshot(ctx, pool, image, name); err != nil {
		resp.Diagnostics.AddError("Create snapshot failed", err.Error())
		return
	}

	// Protect if requested
	if plan.Protected.ValueBool() {
		if err := r.client.ProtectSnapshot(ctx, pool, image, name); err != nil {
			resp.Diagnostics.AddError("Protect snapshot failed", err.Error())
			return
		}
	}

	// Metadata not available in this go-ceph version
	plan.CreatedAt = types.StringValue("")
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s@%s", pool, image, name))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *rbdSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state snapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool := state.Pool.ValueString()
	image := state.Image.ValueString()
	name := state.Name.ValueString()

	exists, err := r.client.SnapshotExists(ctx, pool, image, name)
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read snapshot failed", err.Error())
		return
	}

	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// No metadata available
	state.CreatedAt = types.StringValue("")

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------
// UPDATE — handle protected=true/false transitions
// ---------------------------------------------------------
func (r *rbdSnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan snapshotModel
	var state snapshotModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pool := plan.Pool.ValueString()
	image := plan.Image.ValueString()
	name := plan.Name.ValueString()

	// Preserve force_delete unless user explicitly changed it
	if plan.ForceDelete.IsUnknown() || plan.ForceDelete.IsNull() {
		// user did NOT set force_delete in config → inherit from state
		plan.ForceDelete = state.ForceDelete
	}
	// else: user set a value → keep plan.ForceDelete as-is

	// Detect protected flag change
	if plan.Protected.ValueBool() != state.Protected.ValueBool() {
		if plan.Protected.ValueBool() {
			// protect snapshot
			if err := r.client.ProtectSnapshot(ctx, pool, image, name); err != nil {
				resp.Diagnostics.AddError("Protect snapshot failed", err.Error())
				return
			}
		} else {
			// unprotect snapshot
			if err := r.client.UnprotectSnapshot(ctx, pool, image, name); err != nil {
				resp.Diagnostics.AddError("Unprotect snapshot failed", err.Error())
				return
			}
		}
	}

	// ID is stable
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s@%s", pool, image, name))

	// created_at is always empty string (Ceph does not expose it)
	if plan.CreatedAt.IsUnknown() || plan.CreatedAt.IsNull() {
		plan.CreatedAt = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// ---------------------------------------------------------
// DELETE — unprotect before delete
// ---------------------------------------------------------
func (r *rbdSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state snapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	pool := state.Pool.ValueString()
	image := state.Image.ValueString()
	name := state.Name.ValueString()
	force := state.ForceDelete.ValueBool()

	// If protected, unprotect first
	if state.Protected.ValueBool() {
		if err := r.client.UnprotectSnapshot(ctx, pool, image, name); err != nil {
			if err := r.client.UnprotectSnapshot(ctx, pool, image, name); err != nil {
				if !force {
					// strict mode → fail
					if !strings.Contains(err.Error(), "not protected") {
						resp.Diagnostics.AddError("Unprotect before delete failed", err.Error())
						return
					}
				}
				// force_delete=true → ignore unprotect errors
			}
		}
	}

	// Now delete snapshot
	if err := r.client.DeleteSnapshot(ctx, pool, image, name); err != nil {
		if !force {
			resp.Diagnostics.AddError("Delete snapshot failed", err.Error())
			return
		}
		// force_delete=true → ignore delete errors
	}
}

func (r *rbdSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: pool/image@snap
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected pool/image@snapshot")
		return
	}

	pool := parts[0]
	rest := parts[1]

	imgParts := strings.SplitN(rest, "@", 2)
	if len(imgParts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected pool/image@snapshot")
		return
	}

	image := imgParts[0]
	snap := imgParts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pool"), pool)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("image"), image)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), snap)...)
}
