package cephclient

import (
	"context"
	"fmt"
	"time"

	"terraform-provider-ceph/internal/rgwadmin"

	"github.com/ceph/go-ceph/rados"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	MonHosts    string
	User        string
	Key         string
	KeyringPath string
	ClusterName string
	Timeout     time.Duration

	// RGW S3 settings (optional)
	RGWEndpoint  string
	RGWAccessKey string
	RGWSecretKey string
	RGWRegion    string
}

type Client struct {
	conn    *rados.Conn
	timeout time.Duration

	// RGW Admin API client (already used elsewhere)
	RGW *rgwadmin.Client

	// RGW S3 settings for AWS SDK v2 S3 client
	rgwEndpoint  string
	rgwAccessKey string
	rgwSecretKey string
	rgwRegion    string
}

func NewClient(cfg Config) (*Client, error) {
	var (
		conn *rados.Conn
		err  error
	)

	if cfg.ClusterName != "" {
		conn, err = rados.NewConnWithClusterAndUser(cfg.ClusterName, cfg.User)
	} else {
		conn, err = rados.NewConnWithUser(cfg.User)
	}
	if err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}

	if cfg.KeyringPath != "" {
		if err := conn.ReadConfigFile(cfg.KeyringPath); err != nil {
			return nil, fmt.Errorf("read keyring: %w", err)
		}
	} else {
		if err := conn.SetConfigOption("key", cfg.Key); err != nil {
			return nil, fmt.Errorf("set key: %w", err)
		}
	}

	if err := conn.SetConfigOption("mon_host", cfg.MonHosts); err != nil {
		return nil, fmt.Errorf("set mon_host: %w", err)
	}

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	region := cfg.RGWRegion
	if region == "" {
		// RGW doesn't really care, but SigV4 requires *some* region.
		region = "us-east-1"
	}

	return &Client{
		conn:         conn,
		timeout:      cfg.Timeout,
		rgwEndpoint:  cfg.RGWEndpoint,
		rgwAccessKey: cfg.RGWAccessKey,
		rgwSecretKey: cfg.RGWSecretKey,
		rgwRegion:    region,
	}, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Shutdown()
	}
}

// withTimeout returns a context with the client’s timeout applied, if set.
func (c *Client) withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, c.timeout)
}

var ErrNotImplemented = fmt.Errorf("not implemented")

// s3ClientForRGW builds an AWS SDK v2 S3 client configured for Ceph RGW.
//
// It uses:
//   - SigV4 signing
//   - Path-style addressing
//   - Custom endpoint (rgwEndpoint)
//   - Static credentials (rgwAccessKey / rgwSecretKey)
func (c *Client) s3ClientForRGW(ctx context.Context) (*s3.Client, error) {
	if c.rgwEndpoint == "" {
		return nil, fmt.Errorf("rgw_endpoint is not configured in provider")
	}
	if c.rgwAccessKey == "" || c.rgwSecretKey == "" {
		return nil, fmt.Errorf("rgw_access_key and rgw_secret_key must be configured for S3 operations")
	}

	creds := credentials.NewStaticCredentialsProvider(c.rgwAccessKey, c.rgwSecretKey, "")

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(c.rgwRegion),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for RGW S3: %w", err)
	}

	awsCfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == s3.ServiceID {
				return aws.Endpoint{
					URL:               c.rgwEndpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		},
	)

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// RGW generally expects path-style addressing.
		o.UsePathStyle = true
	})

	return s3Client, nil
}

func (c *Client) Timeout() time.Duration {
	return c.timeout
}
