package s3

import (
	"context"
	"fmt"

	"terraform-provider-ceph/internal/cephclient"
	"terraform-provider-ceph/internal/rgwadmin"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3UserQuotaResource struct {
	client *cephclient.Client
}

func NewS3UserQuotaResource() resource.Resource {
	return &S3UserQuotaResource{}
}

func (r *S3UserQuotaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_user_quota"
}

func (r *S3UserQuotaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3UserQuotaResourceSchema()
}

func (r *S3UserQuotaResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*cephclient.Client)
	}
}

func quotaSpecFromModel(m *S3QuotaModel) rgwadmin.QuotaSpec {
	if m == nil {
		return rgwadmin.QuotaSpec{}
	}
	return rgwadmin.QuotaSpec{
		Enabled:    m.Enabled.ValueBool(),
		MaxSizeKb:  m.MaxSizeKb.ValueInt64(),
		MaxObjects: m.MaxObjects.ValueInt64(),
	}
}

func quotaModelFromSpec(q *rgwadmin.QuotaSpec) *S3QuotaModel {
	if q == nil {
		return &S3QuotaModel{}
	}
	return &S3QuotaModel{
		Enabled:    types.BoolValue(q.Enabled),
		MaxSizeKb:  types.Int64Value(q.MaxSizeKb),
		MaxObjects: types.Int64Value(q.MaxObjects),
	}
}

func (r *S3UserQuotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3UserQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil || r.client.RGW == nil {
		resp.Diagnostics.AddError("Provider not configured", "RGW admin client missing")
		return
	}

	qs := quotaSpecFromModel(plan.Quota)
	if err := r.client.RGW.SetUserQuota(ctx, plan.UID.ValueString(), qs); err != nil {
		resp.Diagnostics.AddError("Create Error", fmt.Sprintf("Failed to set user quota: %s", err))
		return
	}

	plan.ID = types.StringValue(plan.UID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3UserQuotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3UserQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	qs, err := r.client.RGW.GetUserQuota(ctx, state.UID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Failed to read user quota: %s", err))
		return
	}

	state.Quota = quotaModelFromSpec(qs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3UserQuotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3UserQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	qs := quotaSpecFromModel(plan.Quota)
	if err := r.client.RGW.SetUserQuota(ctx, plan.UID.ValueString(), qs); err != nil {
		resp.Diagnostics.AddError("Update Error", fmt.Sprintf("Failed to update user quota: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3UserQuotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3UserQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	// Disable quota on delete
	_ = r.client.RGW.SetUserQuota(ctx, state.UID.ValueString(), rgwadmin.QuotaSpec{})

}

func (r *S3UserQuotaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uid"), req.ID)...)
}
