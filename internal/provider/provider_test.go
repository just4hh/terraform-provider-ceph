package provider

import (
	"syscall"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var testAccProviderFactories = map[string]func() (interface{}, error){
	"ceph": func() (interface{}, error) {
		return providerserver.NewProtocol6(New()), nil
	},
}

func testAccPreCheck(t *testing.T) {
	required := []string{
		"CEPH_MON_HOSTS",
		"CEPH_USER",
		"CEPH_KEY",
	}

	for _, v := range required {
		if val := getenv(v); val == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func getenv(k string) string {
	v, _ := syscall.Getenv(k)
	return v
}
