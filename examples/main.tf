terraform {
  required_providers {
    ceph = {
      source  = "just4hh/ceph"
      version = "0.0.3"
    }
  }
}

provider "ceph" {
  mon_hosts = "ceph_mon"
  user      = "admin"
  key       = "admin_key"
  
  rgw_endpoint   = "http://s3"
  rgw_access_key = "admin_access_key"
  rgw_secret_key = "admin_secret_key"
}

resource "ceph_pool" "rbd" {
  name           = "rbd"
  pg_num         = 8
  size           = 3
  min_size       = 2
  application    = "rbd"
  autoscale_mode = "on"
}

# # # ########################################### # #
# # # # RBD image in the rbd pool               # # #
# # # ########################################### # #

resource "ceph_rbd_image" "example" {
  depends_on = [ceph_pool.rbd]

  name = "tf-test"
  pool = ceph_pool.rbd.name
  size = 2147483648
}

# # # ########################################### # #
# # # # RBD image in the tf-pool pool           # # #
# # # ########################################### # #

resource "ceph_rbd_snapshot" "test" {
  depends_on = [ceph_rbd_image.example]
  pool = "rbd"
  image = "tf-test"
  name = "snap1"
  protected = false
  force_delete = false
}

# # # ########################################### # #
# # # # Cephx user                              # # #
# # # ########################################### # #

resource "ceph_user" "backup" {
  name = "client.backup"

  caps = {
    mon = "allow r"
    osd = "allow rwx pool=rbd"
    mgr = "allow r"
  }
  # rotation_trigger = timestamp() 
}

# # ########################################### # #
# # # S3 user                                 # # #
# # ########################################### # #

resource "ceph_s3_user" "backup" {
  uid          = "backup"
  display_name = "Backup User"
  email        = "backup@example.com"
  suspended    = false
}

resource "ceph_s3_user_key" "backup" {
  depends_on = [ceph_s3_user.backup]
  user_id          = ceph_s3_user.backup.uid
  key_version_id = "13"
}

# resource "ceph_s3_user_key" "backupb" {
#   depends_on = [ceph_s3_user.backup]
#   user_id          = ceph_s3_user.backup.uid
#   key_version_id = "5"
# }

# # ########################################### # #
# # # S3 bucket + object                      # # #
# # ########################################### # #

resource "ceph_s3_bucket" "b" {
  name = "acl-test"
  versioning {
    enabled = true
  }
  force_destroy = true
}

resource "ceph_s3_bucket_acl" "acl" {
  depends_on = [ceph_s3_bucket.b]
  bucket = ceph_s3_bucket.b.name

  grant {
    type       = "CanonicalUser"
    id         = "backup"
    permission = "FULL_CONTROL"
  }
}

resource "ceph_s3_object" "obj" {
  depends_on = [ceph_s3_bucket.b]
  bucket = ceph_s3_bucket.b.name
  key    = "hello.txt"
  content_type = "text/plain"
  body   = "hello world"
}

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

resource "ceph_s3_bucket" "c" {
  name = "mybucket"

  versioning {
    enabled = true
  }
}

resource "ceph_s3_user_quota" "q" {
  uid = ceph_s3_user.backup.uid

  quota {
    enabled     = true
    max_size_kb = 1000000
    max_objects = 10000
  }
}

resource "ceph_s3_bucket_quota" "bq" {
  uid    = ceph_s3_user.backup.uid
  bucket = ceph_s3_bucket.c.name

  quota {
    enabled     = true
    max_size_kb = 500000
    max_objects = 5000
  }
}

# # ########################################### # #
# # # OSD                                     # # #
# # ########################################### # #

##  resource "ceph_osd" "osd4" {
##    osd_id = 4
##  }
