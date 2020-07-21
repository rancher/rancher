package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	if err := os.RemoveAll("./pkg/client/generated"); err != nil {
		return err
	}
	if err := os.RemoveAll("./pkg/generated"); err != nil {
		return err
	}
	return filepath.Walk("./pkg/apis", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, "vendor") {
			return filepath.SkipDir
		}

		if strings.HasPrefix(info.Name(), "zz_generated") {
			fmt.Println("Removing", path)
			if err := os.Remove(path); err != nil {
				return err
			}
		}

		return nil
	})
}
