package cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/agent/steve"
)

var running bool

func RunControllers(ctx context.Context) error {
	if running {
		return nil
	}

	if err := steve.Run(ctx); err != nil {
		return err
	}

	running = true
	return nil
}
