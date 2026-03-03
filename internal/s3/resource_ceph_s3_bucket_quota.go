package s3

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-ceph/internal/cephclient"
	"terraform-provider-ceph/internal/rgwadmin"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3BucketQuotaResource struct {
	client *cephclient.Client
}

func NewS3BucketQuotaResource() resource.Resource {
	return &S3BucketQuotaResource{}
}

func (r *S3BucketQuotaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_quota"
}

func (r *S3BucketQuotaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3BucketQuotaResourceSchema()
}

func (r *S3BucketQuotaResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*cephclient.Client)
	}
}

func (r *S3BucketQuotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	qs := quotaSpecFromModel(plan.Quota)
	if err := r.client.RGW.SetBucketQuota(ctx, plan.UID.ValueString(), plan.Bucket.ValueString(), qs); err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Failed to set bucket quota: %s", err))
		return
	}

	plan.ID = types.StringValue(plan.UID.ValueString() + "/" + plan.Bucket.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketQuotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	qs, err := r.client.RGW.GetBucketQuota(ctx, state.UID.ValueString(), state.Bucket.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Failed to read bucket quota: %s", err))
		return
	}

	state.Quota = quotaModelFromSpec(qs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketQuotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	qs := quotaSpecFromModel(plan.Quota)
	if err := r.client.RGW.SetBucketQuota(ctx, plan.UID.ValueString(), plan.Bucket.ValueString(), qs); err != nil {
		resp.Diagnostics.AddError("Update Error", fmt.Sprintf("Failed to update bucket quota: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketQuotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	_ = r.client.RGW.SetBucketQuota(ctx, state.UID.ValueString(), state.Bucket.ValueString(), rgwadmin.QuotaSpec{})
}

func (r *S3BucketQuotaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: <uid>/<bucket>")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uid"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), parts[1])...)
}
