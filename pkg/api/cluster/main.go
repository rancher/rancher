package main

import (
	"context"
	"os"

	"fmt"
	"net/http"

	"github.com/rancher/rancher/pkg/api/cluster/server"
	"github.com/rancher/types/config"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}

	app, err := config.NewClusterContext(*kubeConfig, *kubeConfig, "local")
	if err != nil {
		return err
	}

	handler, err := server.New(context.Background(), app)
	if err != nil {
		return err
	}

	fmt.Println("Listening on 0.0.0.0:1234")
	return http.ListenAndServe("0.0.0.0:1234", handler)
}
