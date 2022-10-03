package corral

import (
	"bytes"
	"io"
	"os/exec"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	debugFlag = "--debug"
)

// SetupCorralConfig sets the corral config vars. It takes a map[string]string as a parameter; the key is the value and the value the value you are setting
// For example we are getting the aws config vars to build a corral from aws.
// results := aws.AWSCorralConfigVars()
// err := corral.SetupCorralConfig(results)
func SetupCorralConfig(configVars map[string]string) error {
	for variable, value := range configVars {
		msg, err := exec.Command("corral", "config", "vars", "set", variable, value).CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "SetupCorralConfig: "+string(msg))
		}
	}
	return nil
}

// CreateCorral creates a corral taking the corral name, the package path, and a debug set so if someone wants to view the
// corral create log
func CreateCorral(corralName, packagePath string, debug bool) (*rest.Config, error) {
	if debug {
		msg, err := exec.Command("corral", "create", corralName, packagePath, debugFlag).CombinedOutput()
		if err != nil {
			return nil, errors.Wrap(err, "CreateCorral: "+string(msg))
		}

		myString := string(msg[:])
		logrus.Infof(myString)
	} else {
		msg, err := exec.Command("corral", "create", corralName, packagePath).CombinedOutput()
		if err != nil {
			return nil, errors.Wrap(err, "CreateCorral: "+string(msg))
		}
	}

	kubeConfig, err := GetKubeConfig(corralName)
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "CreateCorral: failed to parse kubeconfig for k3d cluster")
	}

	return restConfig, nil
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
