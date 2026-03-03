# Internal Docs


<!-- # run unit tests
go test ./internal/cephclient -v -->

# build provider
cd cmd/terraform-provider-ceph
go build -o terraform-provider-ceph

# install into Terraform local provider path
OS_ARCH="linux_amd64"
DEST="$HOME/terraform/providers/registry.terraform.io/just4hh/ceph/0.0.2/$OS_ARCH"
mkdir -p "$DEST"
cp terraform-provider-ceph "$DEST/terraform-provider-ceph"
chmod +x "$DEST/terraform-provider-ceph"

# re-init and apply in your example
cd ../../examples/rbd_image
rm -rf .terraform .terraform.lock.hcl
terraform init -upgrade -reconfigure
TF_LOG=DEBUG TF_LOG_PATH=./tf.log terraform apply -auto-approve 2>&1 | tee provider-run.log
