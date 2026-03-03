package cephclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3ACL represents a normalized ACL for a bucket or object.
type S3ACL struct {
	OwnerID   string
	OwnerName string
	Grants    []S3Grant
	// CannedACL is optional; currently not used in Put* calls, but kept for future extension.
	CannedACL string
}

type S3Grant struct {
	GranteeType string // "CanonicalUser" or "Group"
	ID          string // Canonical user ID (for CanonicalUser)
	URI         string // Group URI (for Group)
	Permission  string // "FULL_CONTROL", "READ", "WRITE", etc.
}

// GetS3BucketACL retrieves the ACL for a bucket.
func (c *Client) GetS3BucketACL(ctx context.Context, bucket string) (*S3ACL, error) {
	s3c, err := c.s3ClientForRGW(ctx)
	if err != nil {
		return nil, err
	}

	out, err := s3c.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: &bucket,
	})
	if err != nil {
		return nil, fmt.Errorf("get bucket acl: %w", err)
	}

	return convertFromAccessControlPolicy(out.Owner, out.Grants), nil
}

// PutS3BucketACL sets the ACL for a bucket.
func (c *Client) PutS3BucketACL(ctx context.Context, bucket string, acl *S3ACL) error {
	s3c, err := c.s3ClientForRGW(ctx)
	if err != nil {
		return err
	}

	owner, grants := convertToAccessControlPolicy(acl)

	_, err = s3c.PutBucketAcl(ctx, &s3.PutBucketAclInput{
		Bucket: &bucket,
		AccessControlPolicy: &s3types.AccessControlPolicy{
			Owner:  owner,
			Grants: grants,
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket acl: %w", err)
	}

	return nil
}

// GetS3ObjectACL retrieves the ACL for an object.
func (c *Client) GetS3ObjectACL(ctx context.Context, bucket, key string) (*S3ACL, error) {
	s3c, err := c.s3ClientForRGW(ctx)
	if err != nil {
		return nil, err
	}

	out, err := s3c.GetObjectAcl(ctx, &s3.GetObjectAclInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("get object acl: %w", err)
	}

	return convertFromAccessControlPolicy(out.Owner, out.Grants), nil
}

// PutS3ObjectACL sets the ACL for an object.
func (c *Client) PutS3ObjectACL(ctx context.Context, bucket, key string, acl *S3ACL) error {
	s3c, err := c.s3ClientForRGW(ctx)
	if err != nil {
		return err
	}

	owner, grants := convertToAccessControlPolicy(acl)

	_, err = s3c.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: &bucket,
		Key:    &key,
		AccessControlPolicy: &s3types.AccessControlPolicy{
			Owner:  owner,
			Grants: grants,
		},
	})
	if err != nil {
		return fmt.Errorf("put object acl: %w", err)
	}

	return nil
}

// convertFromAccessControlPolicy converts AWS SDK ACL structures into our internal S3ACL.
func convertFromAccessControlPolicy(owner *s3types.Owner, grants []s3types.Grant) *S3ACL {
	acl := &S3ACL{
		Grants: make([]S3Grant, 0, len(grants)),
	}

	if owner != nil {
		if owner.ID != nil {
			acl.OwnerID = *owner.ID
		}
		if owner.DisplayName != nil {
			acl.OwnerName = *owner.DisplayName
		}
	}

	for _, g := range grants {
		var granteeType, id, uri string

		if g.Grantee != nil {
			if g.Grantee.Type != "" {
				granteeType = string(g.Grantee.Type)
			}
			if g.Grantee.ID != nil {
				id = *g.Grantee.ID
			}
			if g.Grantee.URI != nil {
				uri = *g.Grantee.URI
			}
		}

		perm := ""
		if g.Permission != "" {
			perm = string(g.Permission)
		}

		acl.Grants = append(acl.Grants, S3Grant{
			GranteeType: granteeType,
			ID:          id,
			URI:         uri,
			Permission:  perm,
		})
	}

	normalizeS3ACL(acl)
	return acl
}

// convertToAccessControlPolicy converts our internal S3ACL into AWS SDK ACL structures.
func convertToAccessControlPolicy(acl *S3ACL) (*s3types.Owner, []s3types.Grant) {
	// Ceph RGW expects Owner.ID to be set and to match the bucket owner UID.
	owner := &s3types.Owner{
		ID:          &acl.OwnerID,
		DisplayName: &acl.OwnerID, // Ceph RGW typically uses UID as DisplayName as well.
	}

	grants := make([]s3types.Grant, 0, len(acl.Grants))
	for _, g := range acl.Grants {
		grant := s3types.Grant{
			Permission: s3types.Permission(g.Permission),
		}

		grantee := &s3types.Grantee{}
		switch g.GranteeType {
		case "CanonicalUser":
			grantee.Type = s3types.TypeCanonicalUser
			if g.ID != "" {
				grantee.ID = &g.ID
			}
		case "Group":
			grantee.Type = s3types.TypeGroup
			if g.URI != "" {
				grantee.URI = &g.URI
			}
		default:
			// leave empty; RGW will reject invalid types, which is fine
		}

		grant.Grantee = grantee
		grants = append(grants, grant)
	}

	return owner, grants
}
