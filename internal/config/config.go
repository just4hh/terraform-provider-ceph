package config

type ProviderConfig struct {
	MonHosts           string  `tfsdk:"mon_hosts"`
	User               string  `tfsdk:"user"`
	Key                *string `tfsdk:"key"`
	KeyringPath        *string `tfsdk:"keyring_path"`
	ClusterName        *string `tfsdk:"cluster_name"`
	Timeout            *string `tfsdk:"timeout"`
	InsecureSkipVerify *bool   `tfsdk:"insecure_skip_verify"`
	RGWEndpoint        string  `tfsdk:"rgw_endpoint"`
	RGWAccessKey       *string `tfsdk:"rgw_access_key"`
	RGWSecretKey       *string `tfsdk:"rgw_secret_key"`
}
