package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rancher/norman/api"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/factory"
	"k8s.io/client-go/tools/clientcmd"
)

type Foo struct {
	types.Resource
	Name     string `json:"name"`
	Foo      string `json:"foo"`
	SubThing Baz    `json:"subThing"`
}

type Baz struct {
	Name string `json:"name"`
}

var (
	version = types.APIVersion{
		Version: "v1",
		Group:   "example.core.cattle.io",
		Path:    "/example/v1",
	}

	Schemas = factory.Schemas(&version)
)

func main() {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		panic(err)
	}

	client, err := proxy.NewClientGetterFromConfig(*kubeConfig)
	if err != nil {
		panic(err)
	}

	crdFactory := crd.Factory{
		ClientGetter: client,
	}

	Schemas.MustImportAndCustomize(&version, Foo{}, func(schema *types.Schema) {
		if err := crdFactory.AssignStores(context.Background(), types.DefaultStorageContext, nil, schema); err != nil {
			panic(err)
		}
	})

	server := api.NewAPIServer()
	if err := server.AddSchemas(Schemas); err != nil {
		panic(err)
	}

	fmt.Println("Listening on 0.0.0.0:1234")
	panic(http.ListenAndServe("0.0.0.0:1234", server))
}
