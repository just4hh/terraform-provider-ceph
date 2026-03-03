package s3

import (
	"context"
	"strings"

	"terraform-provider-ceph/internal/cephclient"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type S3ObjectACLResource struct {
	client *cephclient.Client
}

type S3ObjectACLGrantModel struct {
	Type       types.String `tfsdk:"type"`
	ID         types.String `tfsdk:"id"`
	URI        types.String `tfsdk:"uri"`
	Permission types.String `tfsdk:"permission"`
}

type S3ObjectACLOwnerModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
}

type S3ObjectACLModel struct {
	ID     types.String            `tfsdk:"id"`
	Bucket types.String            `tfsdk:"bucket"`
	Key    types.String            `tfsdk:"key"`
	Owner  *S3ObjectACLOwnerModel  `tfsdk:"owner"`
	Grants []S3ObjectACLGrantModel `tfsdk:"grant"`
}

func NewS3ObjectACLResource() resource.Resource {
	return &S3ObjectACLResource{}
}

func (r *S3ObjectACLResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_object_acl"
}

func (r *S3ObjectACLResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3ObjectACLResourceSchema()
}

func (r *S3ObjectACLResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*cephclient.Client)
	}
}

func (r *S3ObjectACLResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3ObjectACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()
	key := plan.Key.ValueString()

	b, err := r.client.ReadS3Bucket(ctx, bucket)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", "failed to read bucket before setting object ACL: "+err.Error())
		return
	}

	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID: "+err.Error())
		return
	}

	acl := expandObjectACL(plan, b.Owner, &resp.Diagnostics, adminUID)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.PutS3ObjectACL(ctx, bucket, key, acl); err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(bucket + "/" + key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3ObjectACLResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3ObjectACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()
	key := state.Key.ValueString()

	acl, err := r.client.GetS3ObjectACL(ctx, bucket, key)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(bucket + "/" + key)
	state.Owner = &S3ObjectACLOwnerModel{
		ID:          types.StringValue(acl.OwnerID),
		DisplayName: types.StringValue(acl.OwnerName),
	}
	state.Grants = flattenObjectACLGrants(acl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3ObjectACLResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3ObjectACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()
	key := plan.Key.ValueString()

	b, err := r.client.ReadS3Bucket(ctx, bucket)
	if err != nil {
		resp.Diagnostics.AddError("Update Error", "failed to read bucket before setting object ACL: "+err.Error())
		return
	}

	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID: "+err.Error())
		return
	}

	acl := expandObjectACL(plan, b.Owner, &resp.Diagnostics, adminUID)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.PutS3ObjectACL(ctx, bucket, key, acl); err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3ObjectACLResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3ObjectACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()
	key := state.Key.ValueString()

	acl, err := r.client.GetS3ObjectACL(ctx, bucket, key)
	if err != nil {
		resp.Diagnostics.AddError("Delete Error", "failed to read object ACL before reset: "+err.Error())
		return
	}

	// Resolve admin UID so we can keep admin FULL_CONTROL even after "reset".
	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID during delete: "+err.Error())
		return
	}

	privateACL := &cephclient.S3ACL{
		OwnerID: acl.OwnerID,
		Grants: []cephclient.S3Grant{
			{
				GranteeType: "CanonicalUser",
				ID:          adminUID,
				Permission:  "FULL_CONTROL",
			},
		},
	}

	if err := r.client.PutS3ObjectACL(ctx, bucket, key, privateACL); err != nil {
		resp.Diagnostics.AddError("Delete Error (reset to private failed)", err.Error())
		return
	}
}

func (r *S3ObjectACLResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: bucket/key")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), parts[1])...)
}

// -------------------------------
// Helpers
// -------------------------------

func expandObjectACL(m S3ObjectACLModel, ownerID string, diags *diag.Diagnostics, adminUID string) *cephclient.S3ACL {
	acl := &cephclient.S3ACL{
		OwnerID: ownerID,
		Grants:  make([]cephclient.S3Grant, 0, len(m.Grants)*2),
	}

	for _, g := range m.Grants {
		acl.Grants = append(acl.Grants, cephclient.S3Grant{
			GranteeType: g.Type.ValueString(),
			ID:          g.ID.ValueString(),
			URI:         strings.ToLower(g.URI.ValueString()),
			Permission:  g.Permission.ValueString(),
		})
	}

	ensure := func(uid string) {
		for _, g := range acl.Grants {
			if g.GranteeType == "CanonicalUser" && g.ID == uid {
				return
			}
		}
		acl.Grants = append(acl.Grants, cephclient.S3Grant{
			GranteeType: "CanonicalUser",
			ID:          uid,
			Permission:  "FULL_CONTROL",
		})
	}

	ensure(ownerID)
	ensure(adminUID)

	return acl
}

func flattenObjectACLGrants(acl *cephclient.S3ACL) []S3ObjectACLGrantModel {
	out := make([]S3ObjectACLGrantModel, 0, len(acl.Grants))
	for _, g := range acl.Grants {
		out = append(out, S3ObjectACLGrantModel{
			Type:       types.StringValue(g.GranteeType),
			ID:         types.StringValue(g.ID),
			URI:        types.StringValue(g.URI),
			Permission: types.StringValue(g.Permission),
		})
	}
	return out
}
