package cmd

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// CreateCommand defines the create command
func CreateCommand() cli.Command {
	return cli.Command{
		Name:            "create",
		Usage:           "Create a kubernetes cluster",
		Action:          createWapper,
		SkipFlagParsing: true,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "driver",
				Usage: "Driver to create kubernetes clusters",
			},
		},
	}
}

func createWapper(ctx *cli.Context) error {
	debug := lookUpDebugFlag()
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	driverName := flagHackLookup("--driver")
	if driverName == "" {
		persistStore := store.CLIPersistStore{}
		// ingore the error as we only care if cluster.name is present
		cls, _ := persistStore.Get(os.Args[len(os.Args)-1])
		if cls.DriverName != "" {
			driverName = cls.DriverName
		} else {
			logrus.Error("Driver name is required")
			return cli.ShowCommandHelp(ctx, "create")
		}
	}
	rpcClient, addr, err := runRPCDriver(driverName)
	if err != nil {
		return err
	}
	driverFlags, err := rpcClient.GetDriverCreateOptions(context.Background())
	if err != nil {
		return err
	}
	flags := getDriverFlags(driverFlags)
	for i, command := range ctx.App.Commands {
		if command.Name == "create" {
			createCmd := &ctx.App.Commands[i]
			createCmd.SkipFlagParsing = false
			createCmd.Flags = append(createCmd.Flags, flags...)
			createCmd.Action = create
		}
	}
	// append plugin addr if it is built-in driver
	if len(os.Args) > 1 && addr != "" {
		args := []string{os.Args[0], "--plugin-listen-addr", addr}
		args = append(args, os.Args[1:len(os.Args)]...)
		return ctx.App.Run(args)
	}
	return ctx.App.Run(os.Args)
}

func flagHackLookup(flagName string) string {
	// e.g. "-d" for "--driver"
	flagPrefix := flagName[1:3]

	// TODO: Should we support -flag-name (single hyphen) syntax as well?
	for i, arg := range os.Args {
		if strings.Contains(arg, flagPrefix) {
			// format '--driver foo' or '-d foo'
			if arg == flagPrefix || arg == flagName {
				if i+1 < len(os.Args) {
					return os.Args[i+1]
				}
			}

			// format '--driver=foo' or '-d=foo'
			if strings.HasPrefix(arg, flagPrefix+"=") || strings.HasPrefix(arg, flagName+"=") {
				return strings.Split(arg, "=")[1]
			}
		}
	}

	return ""
}

type cliConfigGetter struct {
	name string
	ctx  *cli.Context
}

func (c cliConfigGetter) GetConfig() (types.DriverOptions, error) {
	driverOpts := getDriverOpts(c.ctx)
	driverOpts.StringOptions["name"] = c.name
	return driverOpts, nil
}

func create(ctx *cli.Context) error {
	persistStore := store.CLIPersistStore{}
	addr := ctx.GlobalString("plugin-listen-addr")
	name := ""
	if ctx.NArg() > 0 {
		name = ctx.Args().Get(0)
	}
	configGetter := cliConfigGetter{
		name: name,
		ctx:  ctx,
	}
	// first try to receive the cluster from disk
	// ingore the error as we only care if cluster.name is present
	clusterFrom, _ := persistStore.Get(os.Args[len(os.Args)-1])
	if clusterFrom.DriverName != "" {
		cls, err := cluster.FromCluster(&clusterFrom, addr, configGetter, persistStore)
		if err != nil {
			return err
		}
		return cls.Create(context.Background())
	}
	// if cluster doesn't exist then we try to create a new one
	driverName := ctx.String("driver")
	if driverName == "" {
		logrus.Error("Driver name is required")
		return cli.ShowCommandHelp(ctx, "create")
	}

	cls, err := cluster.NewCluster(driverName, name, addr, configGetter, persistStore)
	if err != nil {
		return err
	}
	if cls.Name == "" {
		logrus.Error("Cluster name is required")
		return cli.ShowCommandHelp(ctx, "create")
	}
	return cls.Create(context.Background())
}

func lookUpDebugFlag() bool {
	for _, arg := range os.Args {
		if arg == "--debug" {
			return true
		}
	}
	return false
}

func getDriverFlags(opts *types.DriverFlags) []cli.Flag {
	flags := []cli.Flag{}
	for k, v := range opts.Options {
		switch v.Type {
		case "int":
			val, err := strconv.Atoi(v.Value)
			if err != nil {
				val = 0
			}
			flags = append(flags, cli.Int64Flag{
				Name:  k,
				Usage: v.Usage,
				Value: int64(val),
			})
		case "intPtr":
			val, err := strconv.Atoi(v.Value)
			if err != nil {
				val = 0
			}
			flags = append(flags, cli.Int64Flag{
				Name:  k,
				Usage: v.Usage,
				Value: int64(val),
			})
		case "string":
			flags = append(flags, cli.StringFlag{
				Name:  k,
				Usage: v.Usage,
				Value: v.Value,
			})
		case "stringSlice":
			flags = append(flags, cli.StringSliceFlag{
				Name:  k,
				Usage: v.Usage,
			})
		case "bool":
			flags = append(flags, cli.BoolFlag{
				Name:  k,
				Usage: v.Usage,
			})
		case "boolPtr":
			flags = append(flags, cli.BoolFlag{
				Name:  k,
				Usage: v.Usage,
			})
		}
	}
	return flags
}
