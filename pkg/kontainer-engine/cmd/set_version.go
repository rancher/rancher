package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func SetVersionCommand() cli.Command {
	return cli.Command{
		Name:      "set-version",
		ShortName: "sv",
		Usage:     "Set the version of Kubernetes",
		Action:    setVersion,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "version",
				Usage: "The version to upgade/downgrade kubernetes to",
			},
		},
	}
}

func setVersion(ctx *cli.Context) error {
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
			return cli.ShowCommandHelp(ctx, "set-version")
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

		if cap.HasSetVersionCapability() {
			err := cluster.SetVersion(context.Background(), &types.KubernetesVersion{Version: ctx.String("version")})

			if err != nil {
				return err
			}

			fmt.Printf("%v updated to %v\n", name, ctx.String("version"))
		} else {
			return fmt.Errorf("no set-version capability available")
		}
	}

	return nil
}
