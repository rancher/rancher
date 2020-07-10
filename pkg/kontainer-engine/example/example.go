package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"context"

	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func main() {
	time.Sleep(time.Second * 2)
	credentialPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	data, err := ioutil.ReadFile(credentialPath)
	if err != nil {
		logrus.Fatal(err)
	}
	b := true
	gkeSpec := &v3.MapStringInterface{
		"projectId":                 "rancher-dev",
		"zone":                      "us-central1-a",
		"nodeCount":                 1,
		"enableKubernetesDashboard": true,
		"enableHttpLoadBalancing":   &b,
		"imageType":                 "ubuntu",
		"enableLegacyAbac":          true,
		"locations":                 []string{"us-central1-a", "us-central1-b"},
		"credential":                string(data),
	}
	spec := v3.ClusterSpec{
		GenericEngineConfig: gkeSpec,
	}

	// You should really implement your own store
	store := store.CLIPersistStore{}
	service := service.NewEngineService(store)

	endpoint, token, cert, err := service.Create(context.Background(), "daishan-test", &v3.KontainerDriver{}, spec)
	if err != nil {
		logrus.Fatal(err)
	}
	fmt.Println(endpoint)
	fmt.Println(token)
	fmt.Println(cert)
	err = service.Remove(context.Background(), "daishan-test", &v3.KontainerDriver{}, spec, true)
	if err != nil {
		logrus.Fatal(err)
	}
}
