package s3

import (
	"context"
	"errors"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3UserResource struct {
	client *cephclient.Client
}

func NewS3UserResource() resource.Resource {
	return &S3UserResource{}
}

func (r *S3UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_user"
}

func (r *S3UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3UserResourceSchema()
}

func (r *S3UserResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*cephclient.Client)
}

func (r *S3UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	user, err := r.client.CreateS3User(
		ctx,
		plan.UID.ValueString(),
		plan.DisplayName.ValueString(),
		plan.Email.ValueString(),
		plan.Suspended.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(user.UID)
	plan.UID = types.StringValue(user.UID)
	plan.DisplayName = types.StringValue(user.DisplayName)
	plan.Email = types.StringValue(user.Email)
	plan.AccessKey = types.StringValue(user.AccessKey)
	plan.SecretKey = types.StringValue(user.SecretKey)
	plan.Suspended = types.BoolValue(user.Suspended)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	user, err := r.client.ReadS3User(ctx, state.UID.ValueString())
	if err != nil {
		if errors.Is(err, cephclient.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(user.UID)
	state.UID = types.StringValue(user.UID)
	state.DisplayName = types.StringValue(user.DisplayName)
	state.Email = types.StringValue(user.Email)
	state.AccessKey = types.StringValue(user.AccessKey)
	state.SecretKey = types.StringValue(user.SecretKey)
	state.Suspended = types.BoolValue(user.Suspended)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3UserModel
	var state S3UserModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider has not been configured.")
		return
	}

	// We treat UID as immutable; if changed, force new in schema instead.
	user := cephclient.S3User{
		UID:         state.UID.ValueString(),
		DisplayName: plan.DisplayName.ValueString(),
		Email:       plan.Email.ValueString(),
		AccessKey:   state.AccessKey.ValueString(),
		SecretKey:   state.SecretKey.ValueString(),
		Suspended:   plan.Suspended.ValueBool(),
	}

	updated, err := r.client.UpdateS3User(ctx, user)
	if err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	plan.ID = types.StringValue(updated.UID)
	plan.UID = types.StringValue(updated.UID)
	plan.DisplayName = types.StringValue(updated.DisplayName)
	plan.Email = types.StringValue(updated.Email)
	plan.AccessKey = types.StringValue(updated.AccessKey)
	plan.SecretKey = types.StringValue(updated.SecretKey)
	plan.Suspended = types.BoolValue(updated.Suspended)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		return
	}

	err := r.client.DeleteS3User(ctx, state.UID.ValueString())
	if err != nil && !errors.Is(err, cephclient.ErrNotFound) {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *S3UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// import by uid
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uid"), req.ID)...)
}
