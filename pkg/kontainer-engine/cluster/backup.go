package cluster

import (
	"context"
	"encoding/json"

	"github.com/rancher/rancher/pkg/kontainer-engine/types"
)

func (c *Cluster) ETCDSave(ctx context.Context, snapshotName string) error {
	driverOpts, err := c.getDriverOps()
	if err != nil {
		return err
	}
	return c.Driver.ETCDSave(ctx, toInfo(c), driverOpts, snapshotName)
}

func (c *Cluster) ETCDRestore(ctx context.Context, snapshotName string) error {
	driverOpts, err := c.getDriverOps()
	if err != nil {
		return err
	}
	info, err := c.Driver.ETCDRestore(ctx, toInfo(c), driverOpts, snapshotName)
	if err != nil {
		return err
	}

	transformClusterInfo(c, info)

	return c.PostCheck(ctx)
}

func (c *Cluster) getDriverOps() (*types.DriverOptions, error) {
	if err := c.restore(); err != nil {
		return nil, err
	}
	driverOpts, err := c.ConfigGetter.GetConfig()
	if err != nil {
		return nil, err
	}

	driverOpts.StringOptions["name"] = c.Name

	for k, v := range c.Metadata {
		if k == "state" {
			state := make(map[string]interface{})
			if err := json.Unmarshal([]byte(v), &state); err == nil {
				flattenIfNotExist(state, &driverOpts)
			}

			continue
		}

		driverOpts.StringOptions[k] = v
	}

	return &driverOpts, nil
}

func (c *Cluster) ETCDRemoveSnapshot(ctx context.Context, snapshotName string) error {
	driverOpts, err := c.getDriverOps()
	if err != nil {
		return err
	}
	return c.Driver.ETCDRemoveSnapshot(ctx, toInfo(c), driverOpts, snapshotName)
}
