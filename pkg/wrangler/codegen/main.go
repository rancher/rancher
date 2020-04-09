package main

import (
	"os"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
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
			"cluster.x-k8s.io": {
				PackageName: "cluster.x-k8s.io",
				Types: []interface{}{
					v1alpha3.Cluster{},
				},
				GenerateClients: true,
			},
		},
	})
}
