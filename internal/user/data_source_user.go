package user

import (
	"context"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceSchema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UserDataSource struct {
	client *cephclient.Client
}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceSchema.Schema{
		Attributes: map[string]datasourceSchema.Attribute{
			"name": datasourceSchema.StringAttribute{
				Required:    true,
				Description: "Ceph user name (e.g. client.rgw).",
			},
			"key": datasourceSchema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Ceph user key.",
			},
			"caps": datasourceSchema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Ceph user capabilities.",
			},
		},
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*cephclient.Client)
}

type UserDataSourceModel struct {
	Name types.String `tfsdk:"name"`
	Key  types.String `tfsdk:"key"`
	Caps types.Map    `tfsdk:"caps"`
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider has not been configured. Ensure the provider block is present and valid.",
		)
		return
	}

	var state UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, err := d.client.ReadUser(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.Key = types.StringValue(info.Key)

	// Convert caps map[string]string → map[string]attr.Value
	capsMap := make(map[string]attr.Value, len(info.Caps))
	for k, v := range info.Caps {
		capsMap[k] = types.StringValue(v)
	}
	state.Caps = types.MapValueMust(types.StringType, capsMap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
