package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/urfave/cli"
)

// RmCommand defines the remove command
func RmCommand() cli.Command {
	return cli.Command{
		Name:      "remove",
		ShortName: "rm",
		Usage:     "Remove kubernetes clusters",
		Action:    rmCluster,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "force,f",
				Usage: "force to remove a cluster",
			},
		},
	}
}

func rmCluster(ctx *cli.Context) error {
	var lastErr error

	for _, name := range ctx.Args() {
		if name == "" || name == "--help" {
			return cli.ShowCommandHelp(ctx, "remove")
		}
		clusters, err := store.GetAllClusterFromStore()
		if err != nil {
			lastErr = err
			continue
		}
		cluster, ok := clusters[name]
		if !ok {
			lastErr = fmt.Errorf("cluster %v can't be found", name)
			continue
		}
		rpcClient, _, err := runRPCDriver(cluster.DriverName)
		if err != nil {
			lastErr = err
			continue
		}
		configGetter := cliConfigGetter{
			name: name,
			ctx:  ctx,
		}
		cluster.ConfigGetter = configGetter
		cluster.PersistStore = store.CLIPersistStore{}
		cluster.Driver = rpcClient
		if err := cluster.Remove(context.Background(), true); err != nil {
			if ctx.Bool("force") {
				cluster.PersistStore.Remove(name)
			} else {
				lastErr = err
				continue
			}
		}

		fmt.Println(cluster.Name)
	}

	return lastErr
}
