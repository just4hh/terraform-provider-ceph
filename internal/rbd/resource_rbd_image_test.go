package rbd_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-ceph/internal/provider"
)

var testAccProviderFactories = map[string]func() (interface{}, error){
	"ceph": func() (interface{}, error) {
		return providerserver.NewProtocol6(provider.New()), nil
	},
}

func testAccPreCheck(t *testing.T) {
	required := []string{
		"CEPH_MON_HOSTS",
		"CEPH_USER",
		"CEPH_KEY",
	}

	for _, v := range required {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func TestAccRBDImage_basic(t *testing.T) {
	resourceName := "ceph_rbd_image.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRBDImageConfig("tf-test", 1<<30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-test"),
					resource.TestCheckResourceAttr(resourceName, "pool", "rbd"),
					resource.TestCheckResourceAttr(resourceName, "size", "1073741824"),
				),
			},
		},
	})
}

func TestAccRBDImage_resize(t *testing.T) {
	resourceName := "ceph_rbd_image.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRBDImageConfig("tf-test", 1<<30),
			},
			{
				Config: testAccRBDImageConfig("tf-test", 2<<30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "size", "2147483648"),
				),
			},
		},
	})
}

func TestAccRBDImage_rename(t *testing.T) {
	resourceName := "ceph_rbd_image.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRBDImageConfig("tf-test", 1<<30),
			},
			{
				Config: testAccRBDImageConfig("tf-test-renamed", 1<<30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-test-renamed"),
				),
			},
		},
	})
}

func TestAccRBDImage_import(t *testing.T) {
	resourceName := "ceph_rbd_image.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRBDImageConfig("tf-test", 1<<30),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccRBDImageConfig(name string, size int64) string {
	return fmt.Sprintf(`
provider "ceph" {
  mon_hosts = "%s"
  user      = "%s"
  key       = "%s"
}

resource "ceph_rbd_image" "test" {
  pool = "rbd"
  name = "%s"
  size = %d
}
`, os.Getenv("CEPH_MON_HOSTS"), os.Getenv("CEPH_USER"), os.Getenv("CEPH_KEY"), name, size)
}
