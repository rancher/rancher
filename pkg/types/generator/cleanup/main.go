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
	return filepath.Walk("./pkg/types", func(path string, info os.FileInfo, err error) error {
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
