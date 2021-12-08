package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func GetClusterSizeCommand() cli.Command {
	return cli.Command{
		Name:      "get-cluster-size",
		ShortName: "gcs",
		Usage:     "Get node count of kubernetes cluster",
		Action:    getClusterSize,
	}
}

func getClusterSize(ctx *cli.Context) error {
	debug := lookUpDebugFlag()
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	clusters, err := store.GetAllClusterFromStore()

	if err != nil {
		return err
	}

	for _, name := range ctx.Args() {
		if name == "" || name == "--help" {
			return cli.ShowCommandHelp(ctx, "get-cluster-size")
		}

		cluster, ok := clusters[name]

		if !ok {
			err = fmt.Errorf("could not find cluster: %v", err)
			logrus.Error(err.Error())

			return err
		}

		rpcClient, _, err := runRPCDriver(cluster.DriverName)

		if err != nil {
			return err
		}

		configGetter := cliConfigGetter{
			name: name,
			ctx:  ctx,
		}

		cluster.ConfigGetter = configGetter
		cluster.PersistStore = store.CLIPersistStore{}
		cluster.Driver = rpcClient

		cap, err := cluster.GetCapabilities(context.Background())
		if err != nil {
			return fmt.Errorf("error getting capabilities: %v", err)
		}

		if cap.HasGetClusterSizeCapability() {
			node, err := cluster.GetClusterSize(context.Background())

			if err != nil {
				return err
			}

			fmt.Printf("%v: %v\n", name, node.Count)
		} else {
			return fmt.Errorf("no get-cluster-size capability available")
		}
	}

	return nil
}
