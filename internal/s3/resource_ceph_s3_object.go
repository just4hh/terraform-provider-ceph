package s3

import (
	"context"
	"errors"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3ObjectResource struct {
	client *cephclient.Client
}

func NewS3ObjectResource() resource.Resource {
	return &S3ObjectResource{}
}

func (r *S3ObjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_object"
}

func (r *S3ObjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3ObjectResourceSchema()
}

func (r *S3ObjectResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *S3ObjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3ObjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	obj, err := r.client.PutS3Object(
		ctx,
		plan.Bucket.ValueString(),
		plan.Key.ValueString(),
		plan.ContentType.ValueString(),
		[]byte(plan.Body.ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Bucket.ValueString() + "/" + plan.Key.ValueString())
	plan.ContentType = types.StringValue(obj.ContentType)
	plan.ETag = types.StringValue(obj.ETag)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3ObjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3ObjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	obj, err := r.client.GetS3Object(ctx, state.Bucket.ValueString(), state.Key.ValueString())
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(state.Bucket.ValueString() + "/" + state.Key.ValueString())
	state.ContentType = types.StringValue(obj.ContentType)
	state.ETag = types.StringValue(obj.ETag)
	state.Body = types.StringValue(string(obj.Body))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3ObjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3ObjectModel
	var state S3ObjectModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	// Name (bucket/key) treated as immutable; schema should ForceNew on change.
	if plan.Body.ValueString() != state.Body.ValueString() ||
		plan.ContentType.ValueString() != state.ContentType.ValueString() {

		obj, err := r.client.PutS3Object(
			ctx,
			state.Bucket.ValueString(),
			state.Key.ValueString(),
			plan.ContentType.ValueString(),
			[]byte(plan.Body.ValueString()),
		)
		if err != nil {
			resp.Diagnostics.AddError("Update Error", err.Error())
			return
		}

		plan.ID = types.StringValue(state.Bucket.ValueString() + "/" + state.Key.ValueString())
		plan.ETag = types.StringValue(obj.ETag)
	} else {
		plan = state
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3ObjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3ObjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	err := r.client.DeleteS3Object(ctx, state.Bucket.ValueString(), state.Key.ValueString())
	if err != nil && !errors.Is(err, cephclient.ErrNotFound) {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *S3ObjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// import ID format: bucket/key
	// split once on first '/'
	for i := 0; i < len(req.ID); i++ {
		if req.ID[i] == '/' {
			bucket := req.ID[:i]
			key := req.ID[i+1:]
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucket)...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), key)...)
			return
		}
	}
	resp.Diagnostics.AddError("Invalid import ID", "Expected format: bucket/key")
}
