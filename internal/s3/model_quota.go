package s3

import "github.com/hashicorp/terraform-plugin-framework/types"

type S3QuotaModel struct {
	Enabled    types.Bool  `tfsdk:"enabled"`
	MaxSizeKb  types.Int64 `tfsdk:"max_size_kb"`
	MaxObjects types.Int64 `tfsdk:"max_objects"`
}

type S3UserQuotaModel struct {
	ID    types.String  `tfsdk:"id"`
	UID   types.String  `tfsdk:"uid"`
	Quota *S3QuotaModel `tfsdk:"quota"`
}

type S3BucketQuotaModel struct {
	ID     types.String  `tfsdk:"id"`
	UID    types.String  `tfsdk:"uid"`
	Bucket types.String  `tfsdk:"bucket"`
	Quota  *S3QuotaModel `tfsdk:"quota"`
}
