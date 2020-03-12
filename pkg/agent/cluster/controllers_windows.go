package cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/agent/steve"
)

func RunControllers(namespace, token, url string) error {
	if err := steve.Run(context.Background(), namespace); err != nil {
		return err
	}
	return nil
}
