package cephclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"terraform-provider-ceph/internal/rgwadmin"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3User struct {
	UID         string
	DisplayName string
	Email       string
	AccessKey   string
	SecretKey   string
	Suspended   bool

	// MUST MATCH rgwadmin.UserInfo.Keys EXACTLY
	Keys []struct {
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
	}
}

type S3Bucket struct {
	Name   string
	Region string
	Owner  string
}

type S3Object struct {
	Bucket      string
	Key         string
	ContentType string
	ETag        string
	Body        []byte
}

type VersioningStatus string

const (
	VersioningEnabled   VersioningStatus = "Enabled"
	VersioningSuspended VersioningStatus = "Suspended"
	VersioningNone      VersioningStatus = ""
)

// -------------------------------
// S3 USER CRUD (unchanged)
// -------------------------------

func (c *Client) CreateS3User(ctx context.Context, uid, displayName, email string, suspended bool) (*S3User, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("RGW client not configured")
	}

	info, err := c.RGW.CreateUser(ctx, uid, displayName, email, suspended)
	if err != nil {
		return nil, err
	}

	// if len(info.Keys) == 0 {
	// 	return nil, fmt.Errorf("RGW returned user without keys")
	// }

	return &S3User{
		UID:         info.UID,
		DisplayName: info.DisplayName,
		Email:       info.Email,
		// AccessKey:   info.Keys[0].AccessKey,
		// SecretKey:   info.Keys[0].SecretKey,
		Suspended: info.Suspended == 1,
	}, nil
}

func (c *Client) ReadS3User(ctx context.Context, uid string) (*S3User, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("RGW client not configured")
	}

	info, err := c.RGW.ReadUser(ctx, uid)
	if err != nil {
		if errors.Is(err, rgwadmin.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	// if len(info.Keys) == 0 {
	// 	return nil, fmt.Errorf("RGW returned user without keys")
	// }
	var AccessKeyValue string
	if info.Keys != nil && len(info.Keys) > 0 {
		AccessKeyValue = info.Keys[0].AccessKey
	}
	var SecretKeyValue string
	if info.Keys != nil && len(info.Keys) > 0 {
		SecretKeyValue = info.Keys[0].SecretKey
	}
	return &S3User{
		UID:         info.UID,
		DisplayName: info.DisplayName,
		Email:       info.Email,
		AccessKey:   AccessKeyValue,
		SecretKey:   SecretKeyValue,
		Suspended:   info.Suspended == 1,
	}, nil
}

func (c *Client) UpdateS3User(ctx context.Context, user S3User) (*S3User, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("RGW client not configured")
	}

	info, err := c.RGW.UpdateUser(ctx, rgwadmin.UserInfo{
		UID:         user.UID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Suspended:   boolToInt(user.Suspended),
	})
	if err != nil {
		return nil, err
	}

	var AccessKeyValue string
	if info.Keys != nil && len(info.Keys) > 0 {
		AccessKeyValue = info.Keys[0].AccessKey
	}
	var SecretKeyValue string
	if info.Keys != nil && len(info.Keys) > 0 {
		SecretKeyValue = info.Keys[0].SecretKey
	}
	return &S3User{
		UID:         info.UID,
		DisplayName: info.DisplayName,
		Email:       info.Email,
		AccessKey:   AccessKeyValue,
		SecretKey:   SecretKeyValue,
		Suspended:   info.Suspended == 1,
	}, nil

}

func (c *Client) DeleteS3User(ctx context.Context, uid string) error {
	if c.RGW == nil {
		return fmt.Errorf("RGW client not configured")
	}
	return c.RGW.DeleteUser(ctx, uid)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// -------------------------------
// S3 USER KEY MANAGEMENT
// -------------------------------

type S3UserKey struct {
	UID       string
	AccessKey string
	SecretKey string
}

func (c *Client) CreateS3UserKey(ctx context.Context, uid string) (*S3UserKey, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("RGW client not configured")
	}

	k, err := c.RGW.CreateUserKey(ctx, uid)
	if err != nil {
		return nil, err
	}

	return &S3UserKey{
		UID:       uid,
		AccessKey: k.AccessKey,
		SecretKey: k.SecretKey,
	}, nil
}

func (c *Client) DeleteS3UserKey(ctx context.Context, uid, accessKey string) error {
	if c.RGW == nil {
		return fmt.Errorf("RGW client not configured")
	}
	return c.RGW.DeleteUserKey(ctx, uid, accessKey)
}

func (c *Client) ListS3UserKeys(ctx context.Context, uid string) ([]S3UserKey, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("RGW client not configured")
	}

	keys, err := c.RGW.ListUserKeys(ctx, uid)
	if err != nil {
		return nil, err
	}

	out := make([]S3UserKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, S3UserKey{
			UID:       uid,
			AccessKey: k.AccessKey,
			SecretKey: k.SecretKey,
		})
	}

	return out, nil
}

// -------------------------------
// S3 BUCKET CRUD
// -------------------------------

// helper: build an S3 client configured for Ceph RGW using the RGW admin credentials
func (c *Client) s3ClientForAdmin() (*s3.Client, error) {
	if c.RGW == nil {
		return nil, fmt.Errorf("rgw client not configured")
	}

	endpoint := c.RGW.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("invalid rgw endpoint: %w", err)
	}

	creds := credentials.NewStaticCredentialsProvider(c.RGW.AccessKey, c.RGW.SecretKey, "")

	awsCfg := aws.Config{
		Region:      "default",
		Credentials: creds,
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					HostnameImmutable: true,
				}, nil
			}),
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	}), nil
}

// CreateS3Bucket creates a bucket via S3 API.
func (c *Client) CreateS3Bucket(ctx context.Context, name string) (*S3Bucket, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return nil, err
	}

	_, err = s3c.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &name,
	})
	if err != nil {
		return nil, fmt.Errorf("create bucket: %w", err)
	}

	_, err = s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &name})
	if err != nil {
		return nil, fmt.Errorf("bucket created but head failed: %w", err)
	}

	// suka!!! xz keep na zavtra
	owner := c.bucketOwnerFromAdmin(ctx, name)

	return &S3Bucket{
		Name:   name,
		Region: "default",
		Owner:  owner,
	}, nil
}

// ReadS3Bucket checks existence and returns bucket info including owner.
func (c *Client) ReadS3Bucket(ctx context.Context, name string) (*S3Bucket, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return nil, err
	}

	_, err = s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &name})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") ||
			strings.Contains(err.Error(), "404") ||
			strings.Contains(err.Error(), "NoSuchBucket") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("head bucket: %w", err)
	}

	owner := c.bucketOwnerFromAdmin(ctx, name)

	return &S3Bucket{
		Name:   name,
		Region: "default",
		Owner:  owner,
	}, nil
}

// GetBucketVersioning returns the versioning status for a bucket.
func (c *Client) GetBucketVersioning(ctx context.Context, name string) (VersioningStatus, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return VersioningNone, err
	}

	out, err := s3c.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: &name,
	})
	if err != nil {
		return VersioningNone, fmt.Errorf("get bucket versioning: %w", err)
	}

	if out.Status == "" {
		return VersioningNone, nil
	}

	switch out.Status {
	case s3types.BucketVersioningStatusEnabled:
		return VersioningEnabled, nil
	case s3types.BucketVersioningStatusSuspended:
		return VersioningSuspended, nil
	default:
		return VersioningStatus(out.Status), nil
	}

}

// PutBucketVersioning sets the versioning status for a bucket.
func (c *Client) PutBucketVersioning(ctx context.Context, name string, status VersioningStatus) error {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return err
	}

	var s string
	switch status {
	case VersioningEnabled:
		s = "Enabled"
	case VersioningSuspended, VersioningNone:
		// Ceph (and AWS) use "Suspended" to disable versioning.
		s = "Suspended"
	default:
		s = string(status)
	}

	_, err = s3c.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: &name,
		VersioningConfiguration: &s3types.VersioningConfiguration{
			Status: s3types.BucketVersioningStatus(s),
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket versioning: %w", err)
	}
	return nil
}

// DeleteS3Bucket deletes the bucket via S3 API.
func (c *Client) DeleteS3Bucket(ctx context.Context, name string) error {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return err
	}

	_, err = s3c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &name})
	if err != nil {
		return fmt.Errorf("delete bucket: %w", err)
	}
	return nil
}

func (c *Client) DeleteS3BucketAdmin(ctx context.Context, name string, purge bool) error {
	if c.RGW == nil {
		return fmt.Errorf("RGW client not configured")
	}
	return c.RGW.DeleteBucket(ctx, name, purge)
}

// -------------------------------
// BUCKET OWNER VIA ADMIN API
// -------------------------------

func (c *Client) bucketOwnerFromAdmin(ctx context.Context, name string) string {
	if c.RGW == nil {
		return ""
	}

	info, err := c.RGW.ReadBucket(ctx, name)
	if err != nil {
		return ""
	}

	return info.Owner
}

// -------------------------------
// S3 OBJECT CRUD (unchanged)
// -------------------------------

func (c *Client) PutS3Object(ctx context.Context, bucket, key, contentType string, body []byte) (S3Object, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return S3Object{}, err
	}

	input := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: &contentType,
	}

	out, err := s3c.PutObject(ctx, input)
	if err != nil {
		return S3Object{}, fmt.Errorf("put object: %w", err)
	}

	etag := ""
	if out.ETag != nil {
		etag = *out.ETag
	}

	return S3Object{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		ETag:        etag,
		Body:        body,
	}, nil
}

func (c *Client) HeadS3Object(ctx context.Context, bucket, key string) (S3Object, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return S3Object{}, err
	}

	out, err := s3c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return S3Object{}, fmt.Errorf("head object: %w", err)
	}

	etag := ""
	if out.ETag != nil {
		etag = *out.ETag
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}

	return S3Object{
		Bucket:      bucket,
		Key:         key,
		ContentType: ct,
		ETag:        etag,
		Body:        nil,
	}, nil
}

func (c *Client) GetS3Object(ctx context.Context, bucket, key string) (S3Object, error) {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return S3Object{}, err
	}

	out, err := s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return S3Object{}, fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return S3Object{}, fmt.Errorf("read object body: %w", err)
	}

	etag := ""
	if out.ETag != nil {
		etag = *out.ETag
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}

	return S3Object{
		Bucket:      bucket,
		Key:         key,
		ContentType: ct,
		ETag:        etag,
		Body:        body,
	}, nil
}

func (c *Client) DeleteS3Object(ctx context.Context, bucket, key string) error {
	s3c, err := c.s3ClientForAdmin()
	if err != nil {
		return err
	}

	_, err = s3c.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
