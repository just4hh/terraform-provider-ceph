package osd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OSDResource{}
var _ resource.ResourceWithConfigure = &OSDResource{}

type OSDResource struct {
	client *cephclient.Client
}

func NewOSDResource() resource.Resource {
	return &OSDResource{}
}

func (r *OSDResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_osd"
}

func (r *OSDResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ResourceSchema()
}

func (r *OSDResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*cephclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *cephclient.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *OSDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OSDResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The Ceph client was not configured on the provider.")
		return
	}

	osdID := plan.OSDID.ValueInt64()
	info, err := r.client.GetOSD(ctx, int(osdID))
	if err != nil {
		if err == cephclient.ErrNotFound {
			resp.Diagnostics.AddError(
				"OSD not found",
				fmt.Sprintf("OSD with id %d does not exist in the cluster", osdID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to read OSD from Ceph",
			err.Error(),
		)
		return
	}

	host, _ := r.client.FindOSDHost(ctx, int(osdID))

	state := OSDResourceModel{
		ID:     types.StringValue(strconv.FormatInt(osdID, 10)),
		OSDID:  types.Int64Value(osdID),
		In:     types.BoolValue(info.In == 1),
		Up:     types.BoolValue(info.Up == 1),
		Weight: types.Float64Value(info.Weight),
		Host:   types.StringValue(host), // host is not guaranteed from osd dump; could be filled later if you add osd tree parsing
	}

	deviceClass, _ := r.client.FindOSDDeviceClass(ctx, int(osdID))
	state.DeviceClass = types.StringValue(deviceClass)

	loc, _ := r.client.FindOSDCrushLocation(ctx, int(osdID))

	state.CrushLocation = types.MapValueMust(
		types.StringType,
		func() map[string]attr.Value {
			m := map[string]attr.Value{}
			for k, v := range loc {
				m[k] = types.StringValue(v)
			}
			return m
		}(),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OSDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OSDResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The Ceph client was not configured on the provider.")
		return
	}

	osdID := state.OSDID.ValueInt64()
	info, err := r.client.GetOSD(ctx, int(osdID))
	if err != nil {
		if err == cephclient.ErrNotFound {
			// OSD no longer exists; remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to read OSD from Ceph",
			err.Error(),
		)
		return
	}

	state.In = types.BoolValue(info.In == 1)
	state.Up = types.BoolValue(info.Up == 1)
	state.Weight = types.Float64Value(info.Weight)

	host, _ := r.client.FindOSDHost(ctx, int(osdID))
	state.Host = types.StringValue(host)

	deviceClass, _ := r.client.FindOSDDeviceClass(ctx, int(osdID))
	state.DeviceClass = types.StringValue(deviceClass)

	loc, _ := r.client.FindOSDCrushLocation(ctx, int(osdID))

	state.CrushLocation = types.MapValueMust(
		types.StringType,
		func() map[string]attr.Value {
			m := map[string]attr.Value{}
			for k, v := range loc {
				m[k] = types.StringValue(v)
			}
			return m
		}(),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OSDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OSDResourceModel
	var state OSDResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The Ceph client was not configured on the provider.")
		return
	}

	osdID := state.OSDID.ValueInt64()

	// Handle in/out changes
	if !plan.In.IsUnknown() && !plan.In.IsNull() && plan.In.ValueBool() != state.In.ValueBool() {
		if err := r.client.SetOSDInOut(ctx, int(osdID), plan.In.ValueBool()); err != nil {
			resp.Diagnostics.AddError(
				"Failed to update OSD in/out state",
				err.Error(),
			)
			return
		}
		state.In = plan.In
	}

	// Handle weight changes
	if !plan.Weight.IsUnknown() && !plan.Weight.IsNull() && plan.Weight.ValueFloat64() != state.Weight.ValueFloat64() {
		if err := r.client.SetOSDWeight(ctx, int(osdID), plan.Weight.ValueFloat64()); err != nil {
			resp.Diagnostics.AddError(
				"Failed to update OSD weight",
				err.Error(),
			)
			return
		}
		state.Weight = plan.Weight
	}

	// Refresh up/in from cluster
	info, err := r.client.GetOSD(ctx, int(osdID))
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to refresh OSD after update",
			err.Error(),
		)
		return
	}
	state.Up = types.BoolValue(info.Up == 1)
	state.In = types.BoolValue(info.In == 1)
	state.Weight = types.Float64Value(info.Weight)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OSDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OSDResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The Ceph client was not configured on the provider.")
		return
	}

	osdID := int(state.OSDID.ValueInt64())

	// Use provider timeout or default to 30 minutes
	wait := 30 * time.Minute
	if r.client.Timeout() > 0 {
		wait = r.client.Timeout()
	}

	if err := r.client.DeleteOSDSafely(ctx, osdID, wait); err != nil {
		resp.Diagnostics.AddError(
			"Failed to safely delete OSD",
			fmt.Sprintf("Error deleting OSD %d: %s", osdID, err),
		)
		return
	}
}
