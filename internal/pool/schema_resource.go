package pool

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func PoolResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a Ceph pool.",
		Attributes: map[string]schema.Attribute{

			"id": schema.StringAttribute{
				Description: "Internal Terraform ID (same as pool name).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": schema.StringAttribute{
				Description: "Name of the pool.",
				Required:    true,
			},

			"pg_num": schema.Int64Attribute{
				Description: "Number of placement groups.",
				// Required:    true,
				// PlanModifiers: []planmodifier.Int64{
				// 	int64planmodifier.UseStateForUnknown(),
				// },
				Optional: true,
				Computed: true,
			},

			"size": schema.Int64Attribute{
				Description: "Replication size of the pool.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},

			"min_size": schema.Int64Attribute{
				Description: "Minimum number of replicas for I/O.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},

			"application": schema.StringAttribute{
				Description: "Application type (e.g. 'rbd').",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"autoscale_mode": schema.StringAttribute{
				Description: "PG autoscale mode (e.g. 'on', 'off', 'warn').",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
