package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeConfig = ".kube/k3s.yaml"
)

func getEmbedded(ctx context.Context) (bool, clientcmd.ClientConfig, error) {
	var (
		err error
	)

	kubeConfig, err := k3sServer(ctx)
	if err != nil {
		return false, nil, err
	}

	_ = os.Setenv("KUBECONFIG", kubeConfig)
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfig}, &clientcmd.ConfigOverrides{})

	return true, clientConfig, nil
}

func k3sServer(ctx context.Context) (string, error) {
	cmd := exec.Command("k3s", "server",
		"--cluster-init",
		"--disable=traefik,servicelb,metrics-server,local-storage",
		"--node-name=local-node",
		"--log=./k3s.log")

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
