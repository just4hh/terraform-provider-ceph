package s3

import (
	"context"
	"errors"
	"strings"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3BucketResource struct {
	client *cephclient.Client
}

func NewS3BucketResource() resource.Resource {
	return &S3BucketResource{}
}

func versioningStatusFromModel(m *S3BucketVersioningModel) cephclient.VersioningStatus {
	if m == nil {
		return cephclient.VersioningNone
	}
	if m.Enabled.IsNull() || m.Enabled.IsUnknown() {
		return cephclient.VersioningNone
	}
	if m.Enabled.ValueBool() {
		return cephclient.VersioningEnabled
	}
	return cephclient.VersioningSuspended
}

func (r *S3BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

func (r *S3BucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3BucketResourceSchema()
}

func (r *S3BucketResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *S3BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	b, err := r.client.CreateS3Bucket(
		ctx,
		plan.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(b.Name)
	plan.Name = types.StringValue(b.Name)
	plan.Region = types.StringValue(b.Region)

	// Apply versioning if requested
	if plan.Versioning != nil {
		status := versioningStatusFromModel(plan.Versioning)
		if err := r.client.PutBucketVersioning(ctx, plan.Name.ValueString(), status); err != nil {
			resp.Diagnostics.AddError("Create Error", "Failed to set bucket versioning: "+err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	b, err := r.client.ReadS3Bucket(ctx, state.Name.ValueString())
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(b.Name)
	state.Name = types.StringValue(b.Name)
	state.Region = types.StringValue(b.Region)

	// Read versioning status
	vStatus, err := r.client.GetBucketVersioning(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read Error", "Failed to read bucket versioning: "+err.Error())
		return
	}
	state.Versioning = versioningModelFromStatus(vStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketModel
	var state S3BucketModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	// Bucket name/region are effectively ForceNew via schema; only versioning is mutable.
	planStatus := versioningStatusFromModel(plan.Versioning)
	stateStatus := versioningStatusFromModel(state.Versioning)

	if planStatus != stateStatus {
		if err := r.client.PutBucketVersioning(ctx, plan.Name.ValueString(), planStatus); err != nil {
			resp.Diagnostics.AddError("Update Error", "Failed to update bucket versioning: "+err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	name := state.Name.ValueString()
	purge := false
	if !state.ForceDestroy.IsNull() && !state.ForceDestroy.IsUnknown() {
		purge = state.ForceDestroy.ValueBool()
	}

	err := r.client.DeleteS3BucketAdmin(ctx, name, purge)
	if err != nil {
		// bucket not empty and force_destroy = false
		if strings.Contains(err.Error(), "BucketNotEmpty") ||
			strings.Contains(err.Error(), "409") {

			resp.Diagnostics.AddError(
				"Bucket Not Empty",
				"The bucket \""+name+"\" is not empty. "+
					"Set force_destroy = true to delete all objects automatically.\n\n"+
					"RGW error: "+err.Error(),
			)
			return
		}

		// not found → treat as success
		if strings.Contains(err.Error(), "NoSuchBucket") ||
			strings.Contains(err.Error(), "404") {
			return
		}

		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *S3BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// import by bucket name
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func versioningModelFromStatus(status cephclient.VersioningStatus) *S3BucketVersioningModel {
	switch status {
	case cephclient.VersioningEnabled:
		return &S3BucketVersioningModel{
			Enabled: types.BoolValue(true),
		}
	case cephclient.VersioningSuspended, cephclient.VersioningNone:
		return &S3BucketVersioningModel{
			Enabled: types.BoolValue(false),
		}
	default:
		// Unknown status; treat as disabled but keep block present.
		return &S3BucketVersioningModel{
			Enabled: types.BoolValue(false),
		}
	}
}
