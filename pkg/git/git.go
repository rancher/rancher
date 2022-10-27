package git

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var (
	controlChars   = regexp.MustCompile("[[:cntrl:]]")
	controlEncoded = regexp.MustCompile("%[0-1][0-9,a-f,A-F]")
)

func Clone(path, url, branch string) error {
	if err := ValidateURL(url); err != nil {
		return err
	}
	return runcmd("git", "clone", "-b", branch, "--single-branch", "--", url, path)
}

func Update(path, commit string) error {
	if err := runcmd("git", "-C", path, "fetch"); err != nil {
		return err
	}
	return runcmd("git", "-C", path, "checkout", commit)
}

func HeadCommit(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	output, err := cmd.Output()
	return strings.Trim(string(output), "\n"), err
}

func RemoteBranchHeadCommit(url, branch string) (string, error) {
	if err := ValidateURL(url); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "ls-remote", "--", url, branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(output))
	}
	parts := strings.Split(string(output), "\t")
	if len(parts) > 0 && len(parts[0]) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("no commit found for url %s branch %s", branch, url)
}

func IsValid(url string) bool {
	if err := ValidateURL(url); err != nil {
		return false
	}
	err := runcmd("git", "ls-remote", "--", url)
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

// FormatURL generates request url if is a private catalog
func FormatURL(pathURL, username, password string) string {
	if len(username) > 0 && len(password) > 0 {
		if u, err := url.Parse(pathURL); err == nil {
			u.User = url.UserPassword(username, password)
			return u.String()
		}
	}
	return pathURL
}

func CloneWithDepth(path, url, branch string, depth int) error {
	if err := ValidateURL(url); err != nil {
		return err
	}
	return runcmd("git", "clone", "-b", branch, "--single-branch", fmt.Sprintf("--depth=%v", depth), "--", url, path)
}

func ValidateURL(pathURL string) error {
	// Don't allow a URL containing control characters, standard or url-encoded
	if controlChars.FindStringIndex(pathURL) != nil || controlEncoded.FindStringIndex(pathURL) != nil {
		return errors.New("Invalid characters in url")
	}
	return nil
}
