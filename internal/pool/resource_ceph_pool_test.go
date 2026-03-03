package pool_test

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
	for _, v := range []string{"CEPH_MON_HOSTS", "CEPH_USER", "CEPH_KEY"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func TestAccPool_basic(t *testing.T) {
	resourceName := "ceph_pool.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPoolConfig("tf-pool", 64),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "tf-pool"),
					resource.TestCheckResourceAttr(resourceName, "pg_num", "64"),
				),
			},
		},
	})
}

func TestAccPool_import(t *testing.T) {
	resourceName := "ceph_pool.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPoolConfig("tf-pool", 64),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPoolConfig(name string, pgNum int) string {
	return fmt.Sprintf(`
provider "ceph" {
  mon_hosts = "%s"
  user      = "%s"
  key       = "%s"
}

resource "ceph_pool" "test" {
  name   = "%s"
  pg_num = %d
}
`, os.Getenv("CEPH_MON_HOSTS"), os.Getenv("CEPH_USER"), os.Getenv("CEPH_KEY"), name, pgNum)
}
