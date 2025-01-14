package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	qasedefaults "github.com/rancher/rancher/tests/v2/validation/pipeline/qase"
	"github.com/sirupsen/logrus"
	qase "go.qase.io/client"
	"gopkg.in/yaml.v2"
)

const (
	runSourceID    = 16
	recurringRunID = 1
)

var (
	testRunName = os.Getenv(qasedefaults.TestRunNameEnvVar)
	qaseToken   = os.Getenv(qasedefaults.QaseTokenEnvVar)
)

type RecurringTestRun struct {
	ID int64 `json:"id" yaml:"id"`
}

func main() {
	// commandline flags
	startRun := flag.Bool("startRun", false, "commandline flag that determines when to start a run, and conversely when to end it.")
	flag.Parse()

	cfg := qase.NewConfiguration()
	cfg.AddDefaultHeader("Token", qaseToken)
	client := qase.NewAPIClient(cfg)

	if *startRun {
		// create test run
		resp, err := createTestRun(client, testRunName)
		if err != nil {
			logrus.Error("error creating test run: ", err)
		}

		newRunID := resp.Result.Id
		recurringTestRun := RecurringTestRun{}
		recurringTestRun.ID = newRunID
		err = writeToConfigFile(recurringTestRun)
		if err != nil {
			logrus.Error("error writiing test run config: ", err)
		}
	} else {

		testRunConfig, err := readConfigFile()
		if err != nil {
			logrus.Fatalf("error reporting converting string to int32: %v", err)
		}
		// complete test run
		_, _, err = client.RunsApi.CompleteRun(context.TODO(), qasedefaults.RancherManagerProjectID, int32(testRunConfig.ID))
		if err != nil {
			log.Fatalf("error completing test run: %v", err)
		}
	}

}

func createTestRun(client *qase.APIClient, testRunName string) (*qase.IdResponse, error) {
	runCreateBody := qase.RunCreate{
		Title: testRunName,
		CustomField: map[string]string{
			fmt.Sprintf("%d", runSourceID): fmt.Sprintf("%d", recurringRunID),
		},
	}

	idResponse, _, err := client.RunsApi.CreateRun(context.TODO(), runCreateBody, qasedefaults.RancherManagerProjectID)
	if err != nil {
		return nil, err
	}

	return &idResponse, nil
}

func writeToConfigFile(config RecurringTestRun) error {
	yamlConfig, err := yaml.Marshal(config)

	if err != nil {
		return err
	}

	return os.WriteFile("testrunconfig.yaml", yamlConfig, 0644)
}

func readConfigFile() (*RecurringTestRun, error) {
	configString, err := os.ReadFile("testrunconfig.yaml")
	if err != nil {
		return nil, err
	}

	var testRunConfig RecurringTestRun
	err = yaml.Unmarshal(configString, &testRunConfig)
	if err != nil {
		return nil, err
	}

	return &testRunConfig, nil
}