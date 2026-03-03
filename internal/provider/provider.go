package provider

import (
	"context"
	"time"

	"terraform-provider-ceph/internal/cephclient"
	"terraform-provider-ceph/internal/config"
	"terraform-provider-ceph/internal/osd"
	"terraform-provider-ceph/internal/pool"
	"terraform-provider-ceph/internal/rbd"
	"terraform-provider-ceph/internal/rgwadmin"
	"terraform-provider-ceph/internal/s3"
	"terraform-provider-ceph/internal/user"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type cephProvider struct {
	client *cephclient.Client
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func New() provider.Provider {
	return &cephProvider{}
}

func (p *cephProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ceph"
}

func (p *cephProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = ProviderSchema()
}

func (p *cephProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg config.ProviderConfig

	// Read provider configuration
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate required fields
	if cfg.MonHosts == "" {
		resp.Diagnostics.AddError("Missing mon_hosts", "The provider requires mon_hosts to be set.")
		return
	}
	if cfg.User == "" {
		resp.Diagnostics.AddError("Missing user", "The provider requires user to be set.")
		return
	}

	// Determine key vs keyring
	key := ""
	keyring := ""
	if cfg.Key != nil && *cfg.Key != "" {
		key = *cfg.Key
	}
	if cfg.KeyringPath != nil && *cfg.KeyringPath != "" {
		keyring = *cfg.KeyringPath
	}
	if key == "" && keyring == "" {
		resp.Diagnostics.AddError(
			"Missing authentication",
			"Either 'key' or 'keyring_path' must be provided.",
		)
		return
	}

	// Parse timeout (optional)
	var timeout time.Duration
	if cfg.Timeout != nil && *cfg.Timeout != "" {
		d, err := time.ParseDuration(*cfg.Timeout)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid timeout",
				"Failed to parse timeout value: "+err.Error(),
			)
			return
		}
		timeout = d
	}

	// Build client config
	clientCfg := cephclient.Config{
		MonHosts:    cfg.MonHosts,
		User:        cfg.User,
		Key:         key,
		KeyringPath: keyring,
		ClusterName: derefString(cfg.ClusterName),
		Timeout:     timeout,

		RGWEndpoint:  cfg.RGWEndpoint,
		RGWAccessKey: derefString(cfg.RGWAccessKey),
		RGWSecretKey: derefString(cfg.RGWSecretKey),
		// Optional: allow a future provider field for region; default is set in NewClient.
		RGWRegion: "",
	}

	client, err := cephclient.NewClient(clientCfg)
	if err != nil {
		resp.Diagnostics.AddError("Failed to connect to Ceph", err.Error())
		return
	}

	// -------------------------------
	// Initialize RGW Admin API client
	// -------------------------------
	if cfg.RGWEndpoint != "" {
		rgw := rgwadmin.New(
			cfg.RGWEndpoint,
			derefString(cfg.RGWAccessKey),
			derefString(cfg.RGWSecretKey),
		)
		client.RGW = rgw
	}

	p.client = client

	// Expose client to resources and data sources
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *cephProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		rbd.NewRBDImageResource,
		rbd.NewRBDSnapshotResource,
		pool.NewPoolResource,
		user.NewUserResource,
		s3.NewS3UserResource,
		s3.NewS3BucketResource,
		s3.NewS3ObjectResource,
		s3.NewS3BucketACLResource,
		s3.NewS3ObjectACLResource,
		s3.NewS3UserQuotaResource,
		s3.NewS3BucketQuotaResource,
		osd.NewOSDResource,
		s3.NewS3UserKeyResource,
	}
}

func (p *cephProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		pool.NewPoolDataSource,
		user.NewUserDataSource,
	}
}
