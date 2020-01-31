package main

import (
	"os"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
)

func main() {
	os.Unsetenv("GOPATH")

	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/rancher/pkg/wrangler/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"management.cattle.io": {
				PackageName: "management.cattle.io",
				Types: []interface{}{
					v3.Cluster{},
					v3.User{},
				},
				GenerateClients: true,
			},
		},
	})
}
