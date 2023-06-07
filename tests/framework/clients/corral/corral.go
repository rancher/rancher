package corral

import (
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
)

const (
	debugFlag       = "--trace"
	skipCleanupFlag = "--skip-cleanup"
)

// GetCorralEnvVar gets corral environment variables
func GetCorralEnvVar(corralName, envVar string) (string, error) {
	msg, err := exec.Command("corral", "vars", corralName, envVar).CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, "GetCorralEnvVar: "+string(msg))
	}

	corralEnvVar := string(msg)
	corralEnvVar = strings.TrimSuffix(corralEnvVar, "\n")
	return corralEnvVar, nil
}

// SetupCorralConfig sets the corral config vars. It takes a map[string]string as a parameter; the key is the value and the value the value you are setting
// For example we are getting the aws config vars to build a corral from aws.
// results := aws.AWSCorralConfigVars()
// err := corral.SetupCorralConfig(results)
func SetupCorralConfig(configVars map[string]string, configUser string, configSSHPath string) error {
	msg, err := exec.Command("corral", "config", "--user_id", configUser, "--public_key", configSSHPath).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Unable to set configuraion: "+string(msg))
	}
	for variable, value := range configVars {
		msg, err := exec.Command("corral", "config", "vars", "set", variable, value).CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "SetupCorralConfig: "+string(msg))
		}
	}
	return nil
}

// SetCustomRepo sets a custom repo for corral to use. It takes a string as a parameter which is the repo you want to use
func SetCustomRepo(repo string) error {
	msg, err := exec.Command("git", "clone", repo, "corral-packages").CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed to git clone remote repo: "+string(msg))
	}
	makemsg, err := exec.Command("make", "init", "-C", "corral-packages").CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed to git clone remote repo: "+string(makemsg))
	}
	makebuildmsg, err := exec.Command("make", "build", "-C", "corral-packages").CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed to git clone remote repo: "+string(makebuildmsg))
	}
	logrus.Infof("Successfully set custom repo: %s", repo)
	return nil
}

// CreateCorral creates a corral taking the corral name, the package path, and a debug set so if someone wants to view the
// corral create log
func CreateCorral(ts *session.Session, corralName, packageName string, debug bool, cleanup bool) ([]byte, error) {
	ts.RegisterCleanupFunc(func() error {
		return DeleteCorral(corralName)
	})

	args := []string{"create"}
	if !cleanup {
		args = append(args, skipCleanupFlag)
	}
	if debug {
		args = append(args, debugFlag)
	}
	args = append(args, corralName, packageName)
	logrus.Infof("Creating corral with the following parameters: %v", args)
	// complicated, but running the command in a way that allows us to
	// capture the output and error(s) and print it to the console
	msg, err := exec.Command("corral", args...).CombinedOutput()
	logrus.Infof("Corral create output: %s", string(msg))
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create corral: "+string(msg))
	}

	return msg, nil
}

// DeleteCorral deletes a corral based on the corral name
func DeleteCorral(corralName string) error {
	msg, err := exec.Command("corral", "delete", corralName).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "DeleteCorral: "+string(msg))
	}
	return nil
}

// ListCorral lists the corrals that currently created
func ListCorral() (map[string]string, error) {
	corralMapList := make(map[string]string)
	msg, err := exec.Command("corral", "list").CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "ListCorral: "+string(msg))
	}
	// The corral list command comes in this format. So the regular
	// expression is we can get the corral name and its state from the commandline
	// +------------+--------+
	// | NAME       | STATUS |
	// +------------+--------+
	// | corralname | READY  |
	// +------------+--------+
	corralNameRegEx := regexp.MustCompile(`\w+ +\| +\w+`)
	corralList := corralNameRegEx.FindAllString(string(msg), -1)
	if len(corralList) == 1 {
		return corralMapList, nil
	}
	for _, corral := range corralList[1:] {
		corralRegEx := regexp.MustCompile(` +\|`)
		corralNameStatus := corralRegEx.Split(corral, -1)
		corralMapList[corralNameStatus[0]] = corralNameStatus[1]
	}
	return corralMapList, nil
}

// GetKubeConfig gets the kubeconfig of corral's cluster
func GetKubeConfig(corral string) ([]byte, error) {
	firstCommand := exec.Command("corral", "vars", corral, "kubeconfig")
	secondCommand := exec.Command("base64", "--decode")

	reader, writer := io.Pipe()
	firstCommand.Stdout = writer
	secondCommand.Stdin = reader

	var byteBuffer bytes.Buffer
	secondCommand.Stdout = &byteBuffer

	err := firstCommand.Start()
	if err != nil {
		return nil, err
	}

	err = secondCommand.Start()
	if err != nil {
		return nil, err
	}

	err = firstCommand.Wait()
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	err = secondCommand.Wait()
	if err != nil {
		return nil, err
	}

	return byteBuffer.Bytes(), nil
}

// UpdateCorralConfig updates a specific corral config var
func UpdateCorralConfig(configVar, value string) error {
	msg, err := exec.Command("corral", "config", "vars", "set", configVar, value).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "SetupCorralConfig: "+string(msg))
	}
	return nil
}

func DeleteAllCorrals() error {
	corralList, err := ListCorral()
	if err != nil {
		return err
	}
	for corralName := range corralList {
		err := DeleteCorral(corralName)
		logrus.Infof("The Corral %s was deleted.", corralName)
		if err != nil {
			return err
		}
	}
	return nil
}
