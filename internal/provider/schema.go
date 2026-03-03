package provider

import (
	providerSchema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

func ProviderSchema() providerSchema.Schema {
	return providerSchema.Schema{
		Attributes: map[string]providerSchema.Attribute{
			"mon_hosts": providerSchema.StringAttribute{
				Required:    true,
				Description: "Comma-separated list of Ceph MON hosts (e.g. 10.0.0.1:6789,10.0.0.2:6789).",
			},
			"user": providerSchema.StringAttribute{
				Required:    true,
				Description: "Ceph client user (e.g. client.admin).",
			},
			"key": providerSchema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Ceph client key. Either key or keyring_path must be set.",
			},
			"keyring_path": providerSchema.StringAttribute{
				Optional:    true,
				Description: "Path to Ceph keyring file. Either key or keyring_path must be set.",
			},
			"cluster_name": providerSchema.StringAttribute{
				Optional:    true,
				Description: "Ceph cluster name (defaults to 'ceph' if empty).",
			},
			"timeout": providerSchema.StringAttribute{
				Optional:    true,
				Description: "Default timeout for Ceph operations (e.g. 30s, 1m).",
			},
			"insecure_skip_verify": providerSchema.BoolAttribute{
				Optional:    true,
				Description: "Skip TLS verification for future RGW/HTTPS integrations.",
			},
			"rgw_endpoint": providerSchema.StringAttribute{
				Optional: true,
			},
			"rgw_access_key": providerSchema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			"rgw_secret_key": providerSchema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}
