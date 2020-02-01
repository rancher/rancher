package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rancher/norman/pkg/kwrapper/etcd"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeConfig = ".kube/k3s.yaml"
)

func getEmbedded(ctx context.Context) (bool, context.Context, *rest.Config, error) {
	var (
		err error
	)

	etcdEndpoints, err := etcd.RunETCD(ctx, "./management-state")
	if err != nil {
		return false, ctx, nil, err
	}

	kubeConfig, err := k3sServer(ctx, etcdEndpoints)
	if err != nil {
		return false, ctx, nil, err
	}

	os.Setenv("KUBECONFIG", kubeConfig)
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfig}, &clientcmd.ConfigOverrides{}).ClientConfig()

	return true, ctx, restConfig, err
}

func k3sServer(ctx context.Context, endpoints []string) (string, error) {
	cmd := exec.Command("k3s", "server",
		"--no-deploy=traefik",
		"--no-deploy=coredns",
		"--no-deploy=servicelb",
		"--no-deploy=metrics-server",
		"--no-deploy=local-storage",
		"--disable-agent",
		fmt.Sprintf("--datastore-endpoint=%s", strings.Join(endpoints, ",")))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go func() {
		err := cmd.Run()
		logrus.Fatalf("k3s exited with: %v", err)
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	kubeConfig := filepath.Join(home, kubeConfig)

	for {
		if _, err := os.Stat(kubeConfig); err == nil {
			return kubeConfig, nil
		}
		logrus.Infof("Waiting for k3s to start")
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("startup interrupted")
		case <-time.After(time.Second):
		}
	}
}
