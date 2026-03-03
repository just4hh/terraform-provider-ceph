package pool

import (
	datasourceSchema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func PoolDataSourceSchema() datasourceSchema.Schema {
	return datasourceSchema.Schema{
		Attributes: map[string]datasourceSchema.Attribute{
			"name": datasourceSchema.StringAttribute{
				Required: true,
			},
			"pg_num": datasourceSchema.Int64Attribute{
				Optional: true,
				Computed: true,
			},
			"size": datasourceSchema.Int64Attribute{
				Computed: true,
			},
			"min_size": datasourceSchema.Int64Attribute{
				Computed: true,
			},
			"application": datasourceSchema.StringAttribute{
				Computed: true,
			},
			"autoscale_mode": datasourceSchema.StringAttribute{
				Computed: true,
			},
		},
	}
}
