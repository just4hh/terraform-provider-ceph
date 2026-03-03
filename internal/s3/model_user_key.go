package s3

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3UserKeyModel struct {
	ID           types.String `tfsdk:"id"`
	UserID       types.String `tfsdk:"user_id"`
	KeyVersionID types.String `tfsdk:"key_version_id"`
	AccessKey    types.String `tfsdk:"access_key"`
	SecretKey    types.String `tfsdk:"secret_key"`
}

// BuildS3UserKeyID builds the Terraform ID for a user key: "<uid>:<access_key>".
func BuildS3UserKeyID(uid, accessKey string) string {
	return fmt.Sprintf("%s:%s", uid, accessKey)
}

// ParseS3UserKeyID parses "<uid>:<access_key>" into its components.
func ParseS3UserKeyID(id string) (string, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid S3 user key ID %q, expected <uid>:<access_key>", id)
	}
	return parts[0], parts[1], nil
}
