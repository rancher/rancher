// +build k8s

package kubectl

import (
	"context"
	"os"

	"github.com/rancher/rancher/pkg/hyperkube"
	"github.com/sirupsen/logrus"
)

func Main() {
	hk := hyperkube.HyperKube{
		Name: "hyperkube",
		Long: "This is an all-in-one binary that can run any of the various Kubernetes servers.",
	}

	hk.AddServer(hyperkube.NewKubectlServer())

	args := os.Args
	if err := hk.Run(args, context.Background().Done()); err != nil {
		logrus.Errorf("%s exited with error: %v", args[0], err)
	}
}
