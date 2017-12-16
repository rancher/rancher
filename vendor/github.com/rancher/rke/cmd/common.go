package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/urfave/cli"
)

func resolveClusterFile(ctx *cli.Context) (string, string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", "", fmt.Errorf("Can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, clusterFile, nil
}
