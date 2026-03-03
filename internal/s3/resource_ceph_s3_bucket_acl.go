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

type S3BucketACLResource struct {
	client *cephclient.Client
}

type S3BucketACLGrantModel struct {
	Type       types.String `tfsdk:"type"`
	ID         types.String `tfsdk:"id"`
	URI        types.String `tfsdk:"uri"`
	Permission types.String `tfsdk:"permission"`
}

type S3BucketACLOwnerModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
}

type S3BucketACLModel struct {
	ID     types.String            `tfsdk:"id"`
	Bucket types.String            `tfsdk:"bucket"`
	Owner  *S3BucketACLOwnerModel  `tfsdk:"owner"`
	Grants []S3BucketACLGrantModel `tfsdk:"grant"`
}

func NewS3BucketACLResource() resource.Resource {
	return &S3BucketACLResource{}
}

func (r *S3BucketACLResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_acl"
}

func (r *S3BucketACLResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = S3BucketACLResourceSchema()
}

func (r *S3BucketACLResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*cephclient.Client)
	}
}

func (r *S3BucketACLResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := plan.Bucket.ValueString()

	// Read bucket to get owner UID
	b, err := r.client.ReadS3Bucket(ctx, bucketName)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", "failed to read bucket: "+err.Error())
		return
	}

	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID: "+err.Error())
		return
	}

	acl := expandBucketACL(plan, b.Owner, &resp.Diagnostics, adminUID)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.PutS3BucketACL(ctx, bucketName, acl); err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	plan.ID = types.StringValue(bucketName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketACLResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := state.Bucket.ValueString()

	acl, err := r.client.GetS3BucketACL(ctx, bucketName)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	state.ID = types.StringValue(bucketName)
	state.Owner = &S3BucketACLOwnerModel{
		ID:          types.StringValue(acl.OwnerID),
		DisplayName: types.StringValue(acl.OwnerName),
	}
	state.Grants = flattenBucketACLGrants(acl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketACLResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketACLModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := plan.Bucket.ValueString()

	b, err := r.client.ReadS3Bucket(ctx, bucketName)
	if err != nil {
		resp.Diagnostics.AddError("Update Error", "failed to read bucket: "+err.Error())
		return
	}

	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID: "+err.Error())
		return
	}

	acl := expandBucketACL(plan, b.Owner, &resp.Diagnostics, adminUID)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.PutS3BucketACL(ctx, bucketName, acl); err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketACLResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketACLModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := state.Bucket.ValueString()

	adminUID, err := r.client.RGW.LookupUserByAccessKey(ctx, r.client.RGW.AccessKey)
	if err != nil {
		resp.Diagnostics.AddError("ACL Error", "Failed to resolve admin UID during delete: "+err.Error())
		return
	}

	privateACL := &cephclient.S3ACL{
		OwnerID: state.Owner.ID.ValueString(),
		Grants: []cephclient.S3Grant{
			{
				GranteeType: "CanonicalUser",
				ID:          adminUID,
				Permission:  "FULL_CONTROL",
			},
		},
	}

	if err := r.client.PutS3BucketACL(ctx, bucketName, privateACL); err != nil {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *S3BucketACLResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), req.ID)...)
}

// -------------------------------
// Helpers
// -------------------------------

func expandBucketACL(m S3BucketACLModel, ownerID string, diags *diag.Diagnostics, adminUID string) *cephclient.S3ACL {
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

func flattenBucketACLGrants(acl *cephclient.S3ACL) []S3BucketACLGrantModel {
	out := make([]S3BucketACLGrantModel, 0, len(acl.Grants))
	for _, g := range acl.Grants {
		out = append(out, S3BucketACLGrantModel{
			Type:       types.StringValue(g.GranteeType),
			ID:         types.StringValue(g.ID),
			URI:        types.StringValue(g.URI),
			Permission: types.StringValue(g.Permission),
		})
	}
	return out
}

func looksLikeAccessKey(s string) bool {
	if len(s) != 20 {
		return false
	}
	return strings.ToUpper(s) == s
}
