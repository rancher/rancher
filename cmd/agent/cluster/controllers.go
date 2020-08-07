package cluster

import (
	"github.com/rancher/rancher/cmd/agent/steve"
)

var running bool

func RunControllers() error {
	if running {
		return nil
	}

	if err := steve.Run(); err != nil {
		return err
	}

	running = true
	return nil
}
