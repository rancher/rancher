package rkeworker

import (
	"context"

	"github.com/rancher/rancher/pkg/rkecerts"
)

func ExecutePlan(ctx context.Context, nodeConfig *NodeConfig) error {
	if nodeConfig.Certs != "" {
		bundle, err := rkecerts.Unmarshal(nodeConfig.Certs)
		if err != nil {
			return err
		}

		if err := bundle.Explode(); err != nil {
			return err
		}
	}

	for name, process := range nodeConfig.Processes {
		if err := runProcess(ctx, name, process); err != nil {
			return err
		}
	}

	return nil
}
