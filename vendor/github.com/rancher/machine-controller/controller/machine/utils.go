package machine

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

var regExHyphen = regexp.MustCompile("([a-z])([A-Z])")

var (
	RegExMachineDirEnv      = regexp.MustCompile("^" + machineDirEnvKey + ".*")
	RegExMachinePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")
	RegExMachineDriverName  = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")
)

const (
	errorCreatingMachine = "Error creating machine: "
	machineDirEnvKey     = "MACHINE_STORAGE_PATH="
	machineCmd           = "docker-machine"
	defaultCattleHome    = "/var/lib/cattle"
	ProvisioningState    = "Provisioning"
	ProvisionedState     = "Provisioned"
	ErrorState           = "Error"
)

func buildBaseHostDir(machineName string) (string, error) {
	machineDir := filepath.Join(getWorkDir(), "machines", machineName)
	return machineDir, os.MkdirAll(machineDir, 0740)
}

func getWorkDir() string {
	workDir := os.Getenv("MACHINE_WORK_DIR")
	if workDir == "" {
		workDir = os.Getenv("CATTLE_HOME")
	}
	if workDir == "" {
		workDir = defaultCattleHome
	}
	return filepath.Join(workDir, "machine")
}

func toMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var dataMap map[string]interface{}
	err = json.Unmarshal(data, &dataMap)
	if err != nil {
		return nil, err
	}
	return dataMap, nil
}

func buildCreateCommand(machine *v3.Machine, configMap map[string]interface{}) []string {
	sDriver := strings.ToLower(machine.Spec.Driver)
	cmd := []string{"create", "-d", sDriver}

	cmd = append(cmd, buildEngineOpts("--engine-install-url", []string{machine.Spec.EngineInstallURL})...)
	cmd = append(cmd, buildEngineOpts("--engine-opt", mapToSlice(machine.Spec.EngineOpt))...)
	cmd = append(cmd, buildEngineOpts("--engine-env", mapToSlice(machine.Spec.EngineEnv))...)
	cmd = append(cmd, buildEngineOpts("--engine-insecure-registry", machine.Spec.EngineInsecureRegistry)...)
	cmd = append(cmd, buildEngineOpts("--engine-label", mapToSlice(machine.Spec.EngineLabel))...)
	cmd = append(cmd, buildEngineOpts("--engine-registry-mirror", machine.Spec.EngineRegistryMirror)...)
	cmd = append(cmd, buildEngineOpts("--engine-storage-driver", []string{machine.Spec.EngineStorageDriver})...)

	for k, v := range configMap {
		dmField := "--" + sDriver + "-" + strings.ToLower(regExHyphen.ReplaceAllString(k, "${1}-${2}"))
		switch v.(type) {
		case int64:
			cmd = append(cmd, dmField, strconv.FormatInt(v.(int64), 10))
		case string:
			cmd = append(cmd, dmField, v.(string))
		case bool:
			if v.(bool) {
				cmd = append(cmd, dmField, strconv.FormatBool(v.(bool)))
			}
		case []string:
			for _, s := range v.([]string) {
				cmd = append(cmd, dmField, s)
			}
		}
	}
	logrus.Debugf("create cmd %v", cmd)
	cmd = append(cmd, machine.Name)
	return cmd
}

func buildEngineOpts(name string, values []string) []string {
	opts := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		opts = append(opts, name, value)
	}
	return opts
}

func mapToSlice(m map[string]string) []string {
	ret := []string{}
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func buildCommand(machineDir string, cmdArgs []string) *exec.Cmd {
	command := exec.Command(machineCmd, cmdArgs...)
	env := initEnviron(machineDir)
	command.Env = env
	return command
}

func initEnviron(machineDir string) []string {
	env := os.Environ()
	found := false
	for idx, ev := range env {
		if RegExMachineDirEnv.MatchString(ev) {
			env[idx] = machineDirEnvKey + machineDir
			found = true
		}
		if RegExMachinePluginToken.MatchString(ev) {
			env[idx] = ""
		}
		if RegExMachineDriverName.MatchString(ev) {
			env[idx] = ""
		}
	}
	if !found {
		env = append(env, machineDirEnvKey+machineDir)
	}
	return env
}

func startReturnOutput(command *exec.Cmd) (io.Reader, io.Reader, error) {
	readerStdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	readerStderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	err = command.Start()
	if err != nil {

		defer readerStdout.Close()
		defer readerStderr.Close()
		return nil, nil, err
	}
	return readerStdout, readerStderr, nil
}

func (m *Lifecycle) reportStatus(stdoutReader io.Reader, stderrReader io.Reader, machine *v3.Machine) error {
	scanner := bufio.NewScanner(stdoutReader)
	for scanner.Scan() {
		msg := scanner.Text()
		logrus.Infof("stdout: %s", msg)
		_, err := filterDockerMessage(msg, machine, false)
		if err != nil {
			return err
		}
		if err := m.updateMachineCondition(machine, v1.ConditionTrue, ProvisioningState, msg); err != nil {
			return err
		}
	}
	scanner = bufio.NewScanner(stderrReader)
	for scanner.Scan() {
		msg := scanner.Text()
		return errors.New(msg)
	}
	return nil
}

func filterDockerMessage(msg string, machine *v3.Machine, errMsg bool) (string, error) {
	if strings.Contains(msg, errorCreatingMachine) || errMsg {
		return "", errors.New(msg)
	}
	if strings.Contains(msg, machine.Name) {
		return "", nil
	}
	return msg, nil
}

func createExtractedConfig(baseDir string, machine *v3.Machine) (string, error) {
	// create the tar.gz file
	destFile := filepath.Join(baseDir, machine.Name+".tar.gz")
	tarfile, err := os.Create(destFile)
	if err != nil {
		return "", err
	}
	defer tarfile.Close()
	fileWriter := gzip.NewWriter(tarfile)
	defer fileWriter.Close()
	tarfileWriter := tar.NewWriter(fileWriter)
	defer tarfileWriter.Close()

	if err := addDirToArchive(baseDir, tarfileWriter); err != nil {
		return "", err
	}

	return destFile, nil
}

func addDirToArchive(source string, tarfileWriter *tar.Writer) error {
	baseDir := filepath.Base(source)

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == source || strings.HasSuffix(info.Name(), ".iso") ||
				strings.HasSuffix(info.Name(), ".tar.gz") ||
				strings.HasSuffix(info.Name(), ".vmdk") ||
				strings.HasSuffix(info.Name(), ".img") {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))

			if err := tarfileWriter.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarfileWriter, file)
			return err
		})
}

func encodeFile(destFile string) (string, error) {
	extractedTarfile, err := ioutil.ReadFile(destFile)
	if err != nil {
		return "", err
	}

	extractedEncodedConfig := base64.StdEncoding.EncodeToString(extractedTarfile)
	if err != nil {
		return "", err
	}

	return extractedEncodedConfig, nil
}

func restoreMachineDir(machine *v3.Machine, baseDir string) error {
	machineBaseDir := filepath.Dir(baseDir)
	if err := os.MkdirAll(machineBaseDir, 0740); err != nil {
		return fmt.Errorf("Error reinitializing config (MkdirAll). Config Dir: %v. Error: %v", machineBaseDir, err)
	}

	if machine.Status.ExtractedConfig == "" {
		return nil
	}

	configBytes, err := base64.StdEncoding.DecodeString(machine.Status.ExtractedConfig)
	if err != nil {
		return fmt.Errorf("Error reinitializing config (base64.DecodeString). Config Dir: %v. Error: %v", machineBaseDir, err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(configBytes))
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("Error reinitializing config (tarRead.Next). Config Dir: %v. Error: %v", machineBaseDir, err)
		}

		filename := header.Name
		filePath := filepath.Join(machineBaseDir, filename)
		logrus.Debugf("Extracting %v", filePath)

		info := header.FileInfo()
		if info.IsDir() {
			err = os.MkdirAll(filePath, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("Error reinitializing config (Mkdirall). Config Dir: %v. Dir: %v. Error: %v", machineBaseDir, info.Name(), err)
			}
			continue
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return fmt.Errorf("Error reinitializing config (OpenFile). Config Dir: %v. File: %v. Error: %v", machineBaseDir, info.Name(), err)
		}
		defer file.Close()
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return fmt.Errorf("Error reinitializing config (Copy). Config Dir: %v. File: %v. Error: %v", machineBaseDir, info.Name(), err)
		}
	}
}

func machineExists(machineDir string, name string) (bool, error) {
	command := buildCommand(machineDir, []string{"ls", "-q"})
	r, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}

	err = command.Start()
	if err != nil {
		return false, err
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		foundName := scanner.Text()
		if foundName == name {
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return false, err
	}

	err = command.Wait()
	if err != nil {
		return false, err
	}

	return false, nil
}

func deleteMachine(machineDir string, machine *v3.Machine) error {
	command := buildCommand(machineDir, []string{"rm", "-f", machine.Name})
	err := command.Start()
	if err != nil {
		return err
	}

	err = command.Wait()
	if err != nil {
		return err
	}

	return nil
}

func getSSHPrivateKey(machineDir string, machine *v3.Machine) (string, error) {
	keyPath := filepath.Join(machineDir, "machines", machine.Name, "id_rsa")
	data, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return "", nil
	}
	return string(data), nil
}

func waitUntilSSHKey(machineDir string, machine *v3.Machine) error {
	keyPath := filepath.Join(machineDir, "machines", machine.Name, "id_rsa")
	startTime := time.Now()
	increments := 1
	for {
		if time.Now().After(startTime.Add(time.Minute * 3)) {
			return errors.New("Timeout waiting for ssh key")
		}
		if _, err := os.Stat(keyPath); err != nil {
			logrus.Debugf("keyPath not found. The machine is probably still provisioning. Sleep %s second", increments)
			time.Sleep(time.Duration(increments) * time.Second)
			increments = increments * 2
			continue
		}
		return nil
	}
}
