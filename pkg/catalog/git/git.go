package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
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

func RemoteBranchHeadCommit(url, branch string) (string, error) {
	cmd := exec.Command("git", "ls-remote", url, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(output))
	}
	parts := strings.Split(string(output), "\t")
	return parts[0], nil
}

func IsValid(url string) bool {
	err := runcmd("git", "ls-remote", url)
	return err == nil
}

func runcmd(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	bufErr := &bytes.Buffer{}
	cmd.Stderr = bufErr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, bufErr.String())
	}
	return nil
}
