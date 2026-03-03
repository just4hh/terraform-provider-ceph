package osd

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ResourceSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform resource ID (stringified OSD ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"osd_id": schema.Int64Attribute{
				Required:    true,
				Description: "Numeric Ceph OSD ID (e.g. 0, 1, 2).",
			},
			"in": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the OSD is 'in' the cluster (true) or 'out' (false).",
			},
			"up": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the OSD is currently 'up' (true) or 'down' (false).",
			},
			"weight": schema.Float64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "CRUSH weight of the OSD.",
			},
			"host": schema.StringAttribute{
				Computed:    true,
				Description: "Host name where the OSD is running (if available).",
			},
			"device_class": schema.StringAttribute{
				Computed:    true,
				Description: "Device class of the OSD (hdd, ssd, nvme).",
			},
			"crush_location": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
