package main

import (
	"context"
	"log"

	"terraform-provider-ceph/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/just4hh/ceph",
	})
	if err != nil {
		log.Fatal(err)
	}
}
