package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func GetVersionCommand() cli.Command {
	return cli.Command{
		Name:      "get-version",
		ShortName: "gv",
		Usage:     "Get the version of Kubernetes",
		Action:    getVersion,
	}
}

func getVersion(ctx *cli.Context) error {
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
			return cli.ShowCommandHelp(ctx, "get-version")
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

		if cap.HasGetVersionCapability() {
			version, err := cluster.GetVersion(context.Background())

			if err != nil {
				return err
			}

			fmt.Printf("%v: %v\n", name, version.Version)
		} else {
			return fmt.Errorf("no get-version capability available")
		}
	}

	return nil
}
