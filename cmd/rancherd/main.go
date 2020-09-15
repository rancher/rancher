package main

import (
	"os"

	"github.com/rancher/rancher/cmd/rancherd/auth"
	"github.com/rancher/rke2/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewServerCommand(),
		cmds.NewAgentCommand(),
		{
			Name:        "reset-admin",
			Usage:       "Bootstrap and reset admin password",
			Description: "Bootstrap and reset admin password",
			Action:      auth.ResetAdmin,
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
