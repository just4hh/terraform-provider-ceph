package user

import (
	resourceSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func UserSchema() resourceSchema.Schema {
	return resourceSchema.Schema{
		Attributes: map[string]resourceSchema.Attribute{
			"id": resourceSchema.StringAttribute{
				Computed: true,
			},

			"name": resourceSchema.StringAttribute{
				Required: true,
			},

			"key": resourceSchema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},

			"caps": resourceSchema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"rotation_trigger": resourceSchema.StringAttribute{
				Optional: true,
			},
		},
	}
}
