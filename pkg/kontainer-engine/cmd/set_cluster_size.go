package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func SetClusterSizeCommand() cli.Command {
	return cli.Command{
		Name:      "set-cluster-size",
		ShortName: "scs",
		Usage:     "Set the node count of Kubernetes cluster",
		Action:    setClusterSize,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "cluster-size",
				Usage: "The cluster-size to upgade/downgrade kubernetes to",
			},
		},
	}
}

func setClusterSize(ctx *cli.Context) error {
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
			return cli.ShowCommandHelp(ctx, "set-cluster-size")
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

		if cap.HasSetClusterSizeCapability() {
			err := cluster.SetClusterSize(context.Background(), &types.NodeCount{Count: ctx.Int64("cluster-size")})

			if err != nil {
				return err
			}

			fmt.Printf("%v updated to %v nodes\n", name, ctx.Int64("cluster-size"))
		} else {
			return fmt.Errorf("no set-cluster-size capability available")
		}
	}

	return nil
}
