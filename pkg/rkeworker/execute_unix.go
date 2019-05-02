// +build !windows

package rkeworker

import (
	"context"
	"strings"
)

func doExecutePlan(ctx context.Context, nodeConfig *NodeConfig, bundleChanged bool) error {
	for name, process := range nodeConfig.Processes {
		if strings.Contains(name, "sidekick") || strings.Contains(name, "share-mnt") {
			if err := runProcess(ctx, name, process, false, false); err != nil {
				return err
			}
		}
	}

	for name, process := range nodeConfig.Processes {
		if !strings.Contains(name, "sidekick") {
			if err := runProcess(ctx, name, process, true, bundleChanged); err != nil {
				return err
			}
		}
	}

	return nil
}
