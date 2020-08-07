package main

import (
	"os"

	"github.com/rancher/rke2/pkg/cli/cmds"
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []*cli.Command{
		cmds.NewServerCommand(),
		cmds.NewAgentCommand(),
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
