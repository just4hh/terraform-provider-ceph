package s3

import (
	"context"
	"errors"
	"sync"
	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3UserKeyResource struct {
	client *cephclient.Client
}

var s3UserKeyMu sync.Mutex

func NewS3UserKeyResource() resource.Resource {
	return &S3UserKeyResource{}
}

func (r *S3UserKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_user_key"
}

func (r *S3UserKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3UserKeyResourceSchema()
}

func (r *S3UserKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *S3UserKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3UserKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	uid := plan.UserID.ValueString()
	if uid == "" {
		resp.Diagnostics.AddError("Invalid user_id", "user_id must be non-empty")
		return
	}

	// Serialize key creation to avoid RGW race
	s3UserKeyMu.Lock()
	key, err := r.client.CreateS3UserKey(ctx, uid)
	s3UserKeyMu.Unlock()

	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.AccessKey = types.StringValue(key.AccessKey)
	plan.SecretKey = types.StringValue(key.SecretKey)
	plan.ID = types.StringValue(BuildS3UserKeyID(uid, key.AccessKey))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3UserKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3UserKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	if state.ID.IsUnknown() || state.ID.IsNull() {
		resp.State.RemoveResource(ctx)
		return
	}

	uid, accessKey, err := ParseS3UserKeyID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		resp.State.RemoveResource(ctx)
		return
	}

	keys, err := r.client.ListS3UserKeys(ctx, uid)
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	found := false
	for _, k := range keys {
		if k.AccessKey == accessKey {
			found = true
			state.UserID = types.StringValue(uid)
			state.AccessKey = types.StringValue(k.AccessKey)
			state.SecretKey = types.StringValue(k.SecretKey)
			break
		}
	}

	if !found {
		// Key no longer exists in RGW; remove from state.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3UserKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All meaningful changes (key_version_id) are ForceNew; no in-place update.
	var plan S3UserKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddError("Update not supported", "S3 user keys are replace-only; changes should trigger recreation.")
}

func (r *S3UserKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3UserKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	if state.ID.IsUnknown() || state.ID.IsNull() {
		return
	}

	uid, accessKey, err := ParseS3UserKeyID(state.ID.ValueString())
	if err != nil {
		// If ID is malformed, nothing safe to delete; just return.
		return
	}

	err = r.client.DeleteS3UserKey(ctx, uid, accessKey)

	if err != nil && !errors.Is(err, cephclient.ErrNotFound) {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *S3UserKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by "<uid>:<access_key>"
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)

	uid, accessKey, err := ParseS3UserKeyID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Expected <uid>:<access_key>")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), uid)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key"), accessKey)...)
}
