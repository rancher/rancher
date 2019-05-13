package common

import (
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/jailer"
)

type HelmPath struct {
	// /opt/jail/<app-name>
	FullPath string
	// /
	InJailPath string
	// /opt/jail/<app-name>/.kubeconfig
	KubeConfigFull string
	// /.kubeconfig
	KubeConfigInJail string
	// /opt/jail/<app-name>/<app-sub>
	AppDirFull string
	// /<app-sub>
	AppDirInJail string
}

func ParseExternalID(externalID string) (string, error) {
	values, err := url.Parse(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Query().Get("catalog")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	return strings.Join([]string{catalog, template, version}, "-"), nil
}

func JailCommand(cmd *exec.Cmd, jailPath string) (*exec.Cmd, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return cmd, nil
	}

	cred, err := jailer.GetUserCred()
	if err != nil {
		return nil, errors.WithMessage(err, "get user cred error")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = cred
	cmd.SysProcAttr.Chroot = jailPath
	return cmd, nil
}
