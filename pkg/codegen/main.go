package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/rancher/rancher/pkg/codegen/generator/cleanup"
)

func main() {
	tempFolder, err := ioutils.TempDir("", "codegen")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempFolder)

	dest := filepath.Join(tempFolder, "src/github.com/rancher/rancher")
	if err := os.MkdirAll(dest, 0755); err != nil {
		panic(err)
	}

	copyCommand := exec.Command("cp", "-r", "./", dest)
	copyCommand.Stdout = os.Stdout
	copyCommand.Stderr = os.Stderr
	if err := copyCommand.Run(); err != nil {
		panic(err)
	}

	goModVendor := exec.Command("go", "mod", "vendor")
	goModVendor.Dir = dest
	goModVendor.Env = append(os.Environ(), "GO111MODULE=on")
	goModVendor.Stdout = os.Stdout
	goModVendor.Stderr = os.Stderr
	if err := goModVendor.Run(); err != nil {
		panic(err)
	}

	if err := cleanup.CleanUp(); err != nil {
		panic(err)
	}

	goGen := exec.Command("go", "run", "pkg/codegen/gen/main.go")
	goGen.Dir = dest
	goGen.Env = append(os.Environ(), fmt.Sprintf("GOPATH=%v", tempFolder), "GO111MODULE=off")
	goGen.Stdout = os.Stdout
	goGen.Stderr = os.Stderr
	if err := goGen.Run(); err != nil {
		panic(err)
	}

	cbCommand := exec.Command("cp", "-r", filepath.Join(dest, "pkg/apis"), filepath.Join(dest, "pkg/client"), filepath.Join(dest, "pkg/generated"), "./pkg")
	cbCommand.Stdout = os.Stdout
	cbCommand.Stderr = os.Stderr
	if err := cbCommand.Run(); err != nil {
		panic(err)
	}

}
