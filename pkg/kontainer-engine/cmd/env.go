package cmd

import (
	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/urfave/cli"
)

// EnvCommand defines the env command
func EnvCommand() cli.Command {
	return cli.Command{
		Name:   "env",
		Usage:  "Set cluster as current context",
		Action: env,
	}
}

func env(ctx *cli.Context) error {
	name := ctx.Args().Get(0)
	if name == "" || name == "--help" {
		return cli.ShowCommandHelp(ctx, "env")
	}

	return store.CLIPersistStore{}.SetEnv(name)
}
