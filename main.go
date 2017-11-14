package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"log"

	managementController "github.com/rancher/cluster-controller/controller"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/server"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	management.LocalConfig = kubeConfig

	handler, err := server.New(context.Background(), management)

	ctx := signal.SigTermCancelContext(context.Background())
	go func() {
		<-ctx.Done()
		if ctx.Err() != nil {
			log.Fatal(ctx.Err())
		}
		os.Exit(1)
	}()

	managementController.Register(ctx, management)

	management.Start(ctx)

	management.Management.Clusters("").Create(&v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
		},
		Spec: v3.ClusterSpec{
			Internal: true,
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on 0.0.0.0:1234")
	return http.ListenAndServe("0.0.0.0:1234", handler)
}
