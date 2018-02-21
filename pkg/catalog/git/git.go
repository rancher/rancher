package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func Clone(path, url, branch string) error {
	return runcmd("git", "clone", "-b", branch, "--single-branch", url, path)
}

func Update(path, branch string) error {
	if err := runcmd("git", "-C", path, "fetch"); err != nil {
		return err
	}
	return runcmd("git", "-C", path, "checkout", fmt.Sprintf("origin/%s", branch))
}

func HeadCommit(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	output, err := cmd.Output()
	return strings.Trim(string(output), "\n"), err
}

func IsValid(url string) bool {
	err := runcmd("git", "ls-remote", url)
	return err == nil
}

func runcmd(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	return cmd.Run()
}
