package cluster

import (
	"context"

	"github.com/rancher/kontainer-engine/types"
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
	return c.Driver.ETCDRestore(ctx, toInfo(c), driverOpts, snapshotName)
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
		driverOpts.StringOptions[k] = v
	}
	return &driverOpts, nil
}
