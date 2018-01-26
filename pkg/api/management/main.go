package main

import (
	"context"
	"net/http"
	"os"

	"github.com/rancher/management-api/server"
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

	management, err := config.NewManagementContext(*kubeConfig)
	if err != nil {
		return err
	}

	ctx := context.Background()

	var handler http.Handler
	handler, err = server.New(ctx, 8080, 8443, management, func() http.Handler {
		return handler
	})
	if err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}
