package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/urfave/cli"
)

func resolveClusterFile(ctx *cli.Context) (string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", fmt.Errorf("Can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)

	/*
		This is a hacky way to add config path to cluster object without messing with
		ClusterUp function and to avoid conflict with calls from kontainer-engine, basically
		i add config path (cluster.yml by default) to a field into the config buffer
		to be parsed later and added as ConfigPath field into cluster object.
	*/
	clusterFileBuff = fmt.Sprintf("%s\nconfig_path: %s\n", clusterFileBuff, clusterFile)
	return clusterFileBuff, nil
}
