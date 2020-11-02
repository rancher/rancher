package main

import (
	"os"

	"github.com/rancher/k3s/pkg/configfilearg"
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
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "password",
					Usage:  "Password for Rancher login",
					EnvVar: "PASSWORD",
				},
				cli.StringFlag{
					Name:   "password-file",
					Usage:  "Password for Rancher login, from file",
					EnvVar: "PASSWORD_FILE",
				},
			},
		},
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil {
		logrus.Fatal(err)
	}
}
