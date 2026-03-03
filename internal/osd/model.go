package osd

import "github.com/hashicorp/terraform-plugin-framework/types"

type OSDResourceModel struct {
	ID            types.String  `tfsdk:"id"`
	OSDID         types.Int64   `tfsdk:"osd_id"`
	In            types.Bool    `tfsdk:"in"`
	Up            types.Bool    `tfsdk:"up"`
	Weight        types.Float64 `tfsdk:"weight"`
	Host          types.String  `tfsdk:"host"`
	DeviceClass   types.String  `tfsdk:"device_class"`
	CrushLocation types.Map     `tfsdk:"crush_location"`
}
