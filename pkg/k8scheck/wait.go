package k8scheck

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Wait(ctx context.Context, config rest.Config) error {
	client, err := kubernetes.NewForConfig(&config)
	if err != nil {
		return err
	}

	for {
		_, err := client.Discovery().ServerVersion()
		if err == nil {
			break
		}
		logrus.Infof("Waiting for server to become available: %v", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("startup canceled")
		case <-time.After(2 * time.Second):
		}
	}

	return nil
}
