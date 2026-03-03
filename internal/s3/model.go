package s3

import "github.com/hashicorp/terraform-plugin-framework/types"

type S3UserModel struct {
	ID          types.String `tfsdk:"id"`
	UID         types.String `tfsdk:"uid"`
	DisplayName types.String `tfsdk:"display_name"`
	Email       types.String `tfsdk:"email"`
	AccessKey   types.String `tfsdk:"access_key"`
	SecretKey   types.String `tfsdk:"secret_key"`
	Suspended   types.Bool   `tfsdk:"suspended"`
}

type S3BucketVersioningModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type S3BucketModel struct {
	ID           types.String             `tfsdk:"id"`
	Name         types.String             `tfsdk:"name"`
	Region       types.String             `tfsdk:"region"`
	ForceDestroy types.Bool               `tfsdk:"force_destroy"`
	Versioning   *S3BucketVersioningModel `tfsdk:"versioning"`
}

type S3ObjectModel struct {
	ID          types.String `tfsdk:"id"`
	Bucket      types.String `tfsdk:"bucket"`
	Key         types.String `tfsdk:"key"`
	ContentType types.String `tfsdk:"content_type"`
	ETag        types.String `tfsdk:"etag"`
	Body        types.String `tfsdk:"body"`
}
