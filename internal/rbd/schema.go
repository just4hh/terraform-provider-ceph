package rbd

import (
	resourceSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func RBDImageSchema() resourceSchema.Schema {
	return resourceSchema.Schema{
		Attributes: map[string]resourceSchema.Attribute{
			"id": resourceSchema.StringAttribute{
				Computed: true,
			},
			"pool": resourceSchema.StringAttribute{
				Required: true,
			},
			"name": resourceSchema.StringAttribute{
				Required: true,
			},
			"size": resourceSchema.Int64Attribute{
				Required: true,
			},
			"features": resourceSchema.ListAttribute{
				Optional:    true,
				Computed:    false, // allow provider to populate value
				ElementType: types.StringType,
			},
		},
	}
}

func RBDSnapshotSchema() resourceSchema.Schema {
	return resourceSchema.Schema{
		Attributes: map[string]resourceSchema.Attribute{
			"id": resourceSchema.StringAttribute{
				Computed:    true,
				Description: "Snapshot ID in the form pool/image@snap",
			},
			"pool": resourceSchema.StringAttribute{
				Required:    true,
				Description: "Pool containing the RBD image.",
			},
			"image": resourceSchema.StringAttribute{
				Required:    true,
				Description: "Name of the RBD image.",
			},
			"name": resourceSchema.StringAttribute{
				Required:    true,
				Description: "Snapshot name.",
			},
			"protected": resourceSchema.BoolAttribute{
				Optional: true,
				Computed: true,
				// default = false
				// allows toggling without ForceNew
				Default:     booldefault.StaticBool(false),
				Description: "Whether the snapshot is protected.",
			},
			"created_at": resourceSchema.StringAttribute{
				Optional: true,
				Computed: true,
				// Default empty string avoids unknown values after apply
				Default:     stringdefault.StaticString(""),
				Description: "Snapshot creation timestamp (not available from Ceph).",
			},
			"force_delete": resourceSchema.BoolAttribute{
				Optional: true,
				Computed: true,
				// Default false: strict deletion
				Default:     booldefault.StaticBool(false),
				Description: "Force deletion even if snapshot is protected or in use.",
			},
		},
	}
}
