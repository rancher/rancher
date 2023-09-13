package helm

import (
	"os/exec"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/tests/framework/pkg/session"
)

var helmCmd = "helm"

// InstallChart installs a helm chart using helm CLI.
// Send the helm set command strings such as "--set", "installCRDs=true"
// in the args argument to be prepended to the helm install command.
func InstallChart(ts *session.Session, releaseName, helmRepo, namespace, version string, args ...string) error {
	// Register cleanup function
	ts.RegisterCleanupFunc(func() error {
		return UninstallChart(releaseName, namespace)
	})

	// Default helm install command
	commandArgs := []string{
		"install",
		releaseName,
		helmRepo,
		"--namespace",
		namespace,
		"--wait",
	}

	commandArgs = append(commandArgs, args...)

	if version != "" {
		commandArgs = append(commandArgs, "--version", version)
	}

	msg, err := exec.Command(helmCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "InstallChart: "+string(msg))
	}

	return nil
}

// UpgradeChart upgrades a helm chart using helm CLI.
// Send the helm set command strings such as "--set", "installCRDs=true"
// in the args argument to be prepended to the helm upgrade command.
func UpgradeChart(ts *session.Session, releaseName, helmRepo, namespace, version string, args ...string) error {
	// Register cleanup function
	ts.RegisterCleanupFunc(func() error {
		return UninstallChart(releaseName, namespace)
	})

	// Default helm upgrade command
	commandArgs := []string{
		"upgrade",
		releaseName,
		helmRepo,
		"--namespace",
		namespace,
		"--wait",
	}

	commandArgs = append(commandArgs, args...)

	if version != "" {
		commandArgs = append(commandArgs, "--version", version)
	}

	msg, err := exec.Command(helmCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "UpgradeChart: "+string(msg))
	}

	return nil
}

// UninstallChart uninstalls a helm chart using helm CLI in a given namespace
// using the releaseName provided.
func UninstallChart(releaseName, namespace string, args ...string) error {
	// Default helm uninstall command
	commandArgs := []string{
		"uninstall",
		releaseName,
		"--namespace",
		namespace,
		"--wait",
	}

	msg, err := exec.Command(helmCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "UninstallChart: "+string(msg))
	}

	return nil
}

// AddHelmRepo adds the specified helm repoistory using the helm repo add command.
func AddHelmRepo(name, url string) error {
	msg, err := exec.Command(helmCmd, "repo", "add", name, url).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "AddHelmRepo: "+string(msg))
	}

	return nil
}
