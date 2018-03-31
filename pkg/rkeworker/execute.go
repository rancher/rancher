package rkeworker

import (
	"context"
	"strings"

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
		if strings.Contains(name, "sidekick") {
			if err := runProcess(ctx, name, process, false); err != nil {
				return err
			}
		}
	}

	for name, process := range nodeConfig.Processes {
		if !strings.Contains(name, "sidekick") {
			if err := runProcess(ctx, name, process, true); err != nil {
				return err
			}
		}
	}

	return nil
}
