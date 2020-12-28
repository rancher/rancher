package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/rancher/rancher/pkg/kontainer-engine/store"
	"github.com/urfave/cli"
)

var inspectHelpTemplate = `{{.Usage}}
{{if .Description}}{{.Description}}{{end}}
Usage: kontainer-engine [global options] {{.Name}} {{if .Flags}}[OPTIONS] {{end}}{{if ne "None" .ArgsUsage}}{{if ne "" .ArgsUsage}}{{.ArgsUsage}}{{else}}[cluster-name]{{end}}{{end}}

{{if .Flags}}Options:{{range .Flags}}
	 {{.}}{{end}}{{end}}
`

// InspectCommand defines the inspect command
func InspectCommand() cli.Command {
	return cli.Command{
		Name:               "inspect",
		Usage:              "inspect kubernetes clusters",
		Action:             inspectCluster,
		Flags:              []cli.Flag{},
		CustomHelpTemplate: inspectHelpTemplate,
	}
}

func inspectCluster(ctx *cli.Context) error {
	name := ctx.Args().Get(0)
	if name == "" {
		return errors.New("name is required when inspecting cluster")
	}
	clusters, err := store.GetAllClusterFromStore()
	if err != nil {
		return err
	}
	cluster, ok := clusters[name]
	if !ok {
		return fmt.Errorf("cluster %v can't be found", name)
	}
	cluster.ClientKey = "Redacted"
	cluster.ClientCertificate = "Redacted"
	cluster.RootCACert = "Redacted"
	data, err := json.MarshalIndent(cluster, "", "\t")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
