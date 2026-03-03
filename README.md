This provider is currently in Beta.
APIs, resource schemas, and behaviors may change between releases.
Use in production environments is not recommended until the interface stabilizes.


Ceph Terraform Provider
Manage Ceph clusters, RBD images, CephX users, and RGW/S3 resources using Terraform.
This provider supports Ceph 19.x (Squid) and integrates with:

Ceph MON / CephX for cluster‑level operations (pools, RBD, users, OSDs)

Ceph RGW (S3 API) signing for buckets, objects, ACLs, and quotas

Features
Manage Ceph pools (replicated, autoscale, application settings)

Create and delete RBD images and snapshots

Manage CephX users and capabilities

Full RGW/S3 support:

Buckets

Objects

ACLs

Quotas

S3 users and keys

Works with standalone Ceph clusters, cephadm, and containerized deployments


Build


```



 CGO_ENABLED=1  go build -a -o terraform-provider-ceph ./cmd/terraform-provider-ceph


```



Installation


```



 cp terraform-provider-ceph /home/username/terraform/providers/registry.terraform.io/just4hh/ceph/0.0.2/linux_amd64/

 terraform init


```




Add the provider to your Terraform configuration:

hcl

```


terraform {
  required_providers {
    ceph = {
      source  = "just4hh/ceph"
      version = "0.0.2"
    }
  }
}

```



Provider Configuration
hcl

```


provider "ceph" {
  mon_hosts = "your-ceph-mon"
  user      = "admin"
  key       = "admin"

  # Optional RGW/S3 configuration
  rgw_endpoint   = "http://ceph-s3"
  rgw_access_key = "YOUR_ACCESS_KEY"
  rgw_secret_key = "YOUR_SECRET_KEY"
}

```



Provider Arguments
Argument	Description	Required
mon_hosts	Comma‑separated list of Ceph MON hosts	Yes
user	CephX user (e.g. client.admin)	Yes
key	CephX key (or use keyring_path)	Conditional
keyring_path	Path to keyring file	Conditional
cluster_name	Ceph cluster name (default: ceph)	No
timeout	Operation timeout (e.g. 30s, 1m)	No
insecure_skip_verify	Skip TLS verification	No
rgw_endpoint	RGW S3 endpoint	No
rgw_access_key	S3 access key	No
rgw_secret_key	S3 secret key	No


Resources

Ceph Pool
Manage Ceph pools.

hcl

```


resource "ceph_pool" "rbd" {
  name           = "rbd"
  pg_num         = 8
  size           = 3
  min_size       = 2
  application    = "rbd"
  autoscale_mode = "on"
}

```



RBD Image
hcl

```


resource "ceph_rbd_image" "example" {
  depends_on = [ceph_pool.rbd]

  name = "tf-test"
  pool = ceph_pool.rbd.name
  size = 2147483648
}

```



RBD Snapshot
hcl

```


resource "ceph_rbd_snapshot" "test" {
  depends_on = [ceph_rbd_image.example]

  pool         = "rbd"
  image        = "tf-test"
  name         = "snap1"
  protected    = false
  force_delete = false
}

```



CephX User
hcl

```


resource "ceph_user" "backup" {
  name = "client.backup"

  caps = {
    mon = "allow r"
    osd = "allow rwx pool=rbd"
    mgr = "allow r"
  }

  rotation_trigger = timestamp()
}

```



rotate cephx user key
rotation_trigger = timestamp() 

RGW / S3 Resources

S3 User

hcl

```


resource "ceph_s3_user" "u" {
  uid          = "demo-user"
  display_name = "Demo User"
}

```



S3 User Key
hcl

```


resource "ceph_s3_user_key" "k" {
  depends_on = [ceph_s3_user.u]

  user_id        = ceph_s3_user.u.uid
  key_version_id = "5"
}

```


ceph s3 user - It is possible to have more than one key pair

S3 Bucket
hcl


```



resource "ceph_s3_bucket" "b" {
  name = "acl-test"

  versioning {
    enabled = true
  }

  force_destroy = true  
}

```


force_destroy
When false (default), deletion fails if bucket contains objects.
When true, deletion uses RGW Admin API purge-objects=true.
  


S3 Bucket ACL
hcl

```


resource "ceph_s3_bucket_acl" "acl" {
  depends_on = [ceph_s3_bucket.b]

  bucket = ceph_s3_bucket.b.name

  grant {
    type       = "CanonicalUser"
    id         = "backup"
    permission = "FULL_CONTROL"
  }
}

```



S3 Object
hcl

```


resource "ceph_s3_object" "obj" {
  depends_on = [ceph_s3_bucket.b]

  bucket = ceph_s3_bucket.b.name
  key    = "hello.txt"
  body   = "hello world"
}

```



S3 Object ACL
hcl

```


resource "ceph_s3_object_acl" "obj_acl" {
  depends_on = [ceph_s3_object.obj]

  bucket = ceph_s3_bucket.b.name
  key    = ceph_s3_object.obj.key

  grant {
    type       = "CanonicalUser"
    id         = "backup"
    permission = "FULL_CONTROL"
  }
}

```



S3 User Quota
hcl

```


resource "ceph_s3_user_quota" "q" {
  uid = ceph_s3_user.backup.uid

  quota {
    enabled     = true
    max_size_kb = 1000000
    max_objects = 10000
  }
}

```



S3 Bucket Quota
hcl

```


resource "ceph_s3_bucket_quota" "bq" {
  uid    = ceph_s3_user.backup.uid
  bucket = ceph_s3_bucket.c.name

  quota {
    enabled     = true
    max_size_kb = 500000
    max_objects = 5000
  }
}

```



OSD Resource 
hcl

```


# resource "ceph_osd" "osd0" {
#   osd_id = 0
# }

```



Data Sources
Pool Lookup

hcl

```


data "ceph_pool" "existing" {
  name = "rbd"
}

```



CephX User Lookup
hcl

```


data "ceph_user" "client_rgw" {
  name = "client.rgw"
}

```



Terraform usage



```


terraform apply


```




```



terraform destroy


```



# Ceph Terraform Provider  
![status](https://img.shields.io/badge/status-beta-orange)

> **Beta Notice:**  
> This provider is under active development. Breaking changes may occur.
