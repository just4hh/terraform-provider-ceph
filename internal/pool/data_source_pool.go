package pool

import (
	"context"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type PoolDataSource struct {
	client *cephclient.Client
}

func NewPoolDataSource() datasource.DataSource {
	return &PoolDataSource{}
}

func (d *PoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (d *PoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = PoolDataSourceSchema()
}

func (d *PoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*cephclient.Client)
}

type PoolDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	PGNum         types.Int64  `tfsdk:"pg_num"`
	Size          types.Int64  `tfsdk:"size"`
	MinSize       types.Int64  `tfsdk:"min_size"`
	Application   types.String `tfsdk:"application"`
	AutoscaleMode types.String `tfsdk:"autoscale_mode"`
}

func (d *PoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured.",
		)
		return
	}

	// ⭐ FIX: declare state BEFORE using it
	var state PoolDataSourceModel

	// Load config into state
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query Ceph
	info, err := d.client.ReadPool(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.PGNum = types.Int64Value(int64(info.PGNum))
	state.Size = types.Int64Value(int64(info.Size))
	state.MinSize = types.Int64Value(int64(info.MinSize))
	state.AutoscaleMode = types.StringValue(info.AutoscaleMode)

	// normalize application metadata
	apps := make([]string, 0)
	for k := range info.ApplicationMetadata {
		apps = append(apps, k)
	}
	if len(apps) > 0 {
		state.Application = types.StringValue(apps[0])
	} else {
		// Preserve existing state to avoid drift
		state.Application = state.Application
	}

	state.ID = types.StringValue(state.Name.ValueString())

	// Save state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
