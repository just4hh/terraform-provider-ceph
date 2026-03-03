package pool

import (
	"context"
	"errors"
	"fmt"
	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type PoolResource struct {
	client *cephclient.Client
}

func NewPoolResource() resource.Resource {
	return &PoolResource{}
}

type PoolModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	PGNum         types.Int64  `tfsdk:"pg_num"`
	Size          types.Int64  `tfsdk:"size"`
	MinSize       types.Int64  `tfsdk:"min_size"`
	Application   types.String `tfsdk:"application"`
	AutoscaleMode types.String `tfsdk:"autoscale_mode"`
}

func (r *PoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (r *PoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = PoolResourceSchema()
}

func (r *PoolResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *PoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	err := r.client.CreatePool(
		ctx,
		plan.Name.ValueString(),
		uint64(plan.PGNum.ValueInt64()),
		uint64(plan.Size.ValueInt64()),
		uint64(plan.MinSize.ValueInt64()),
		plan.Application.ValueString(),
		plan.AutoscaleMode.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	info, err := r.client.ReadPool(ctx, state.Name.ValueString())
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	// --- Safe normalization ---
	if state.AutoscaleMode.ValueString() == "off" {
		state.PGNum = types.Int64Value(int64(info.PGNum))
	}
	// else: keep state.PGNum unchanged
	state.Size = types.Int64Value(int64(info.Size))
	state.MinSize = types.Int64Value(int64(info.MinSize))

	// AutoscaleMode: preserve state if Ceph returns empty
	if info.AutoscaleMode != "" {
		state.AutoscaleMode = types.StringValue(info.AutoscaleMode)
	}

	// Application: Ceph may return empty metadata even when app is enabled
	if len(info.ApplicationMetadata) > 0 {
		for k := range info.ApplicationMetadata {
			state.Application = types.StringValue(k)
			break
		}
	}
	// else: keep existing state.Application to avoid drift

	state.ID = types.StringValue(state.Name.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PoolModel
	var state PoolModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	name := plan.Name.ValueString()

	// --- Update size ---
	if plan.Size.ValueInt64() != state.Size.ValueInt64() {
		if err := r.client.SetPoolProperty(ctx, name, "size", fmt.Sprintf("%d", plan.Size.ValueInt64())); err != nil {
			resp.Diagnostics.AddError("Update size error", err.Error())
			return
		}
	}

	// --- Update min_size ---
	if plan.MinSize.ValueInt64() != state.MinSize.ValueInt64() {
		if err := r.client.SetPoolProperty(ctx, name, "min_size", fmt.Sprintf("%d", plan.MinSize.ValueInt64())); err != nil {
			resp.Diagnostics.AddError("Update min_size error", err.Error())
			return
		}
	}

	// --- Update pg_num ---
	// Skip pg_num updates when autoscale is on
	if plan.AutoscaleMode.ValueString() == "off" {
		if plan.PGNum.ValueInt64() != state.PGNum.ValueInt64() {
			err := r.client.SetPoolProperty(ctx, name, "pg_num",
				fmt.Sprintf("%d", plan.PGNum.ValueInt64()))
			if err != nil {
				resp.Diagnostics.AddError("Update pg_num error", err.Error())
				return
			}
		}
	}

	// --- Update autoscale ---
	if plan.AutoscaleMode.ValueString() != state.AutoscaleMode.ValueString() {
		if err := r.client.SetPoolProperty(ctx, name, "pg_autoscale_mode", plan.AutoscaleMode.ValueString()); err != nil {
			resp.Diagnostics.AddError("Update autoscale_mode error", err.Error())
			return
		}
	}

	// --- Update application ---
	if plan.Application.ValueString() != state.Application.ValueString() {
		if err := r.client.SetPoolApplication(ctx, name, plan.Application.ValueString()); err != nil {
			resp.Diagnostics.AddError("Update application error", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	err := r.client.DeletePool(ctx, state.Name.ValueString())
	if err != nil && !errors.Is(err, cephclient.ErrNotFound) {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *PoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
