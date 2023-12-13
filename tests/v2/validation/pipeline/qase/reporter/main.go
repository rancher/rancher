package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/antihax/optional"
	qasedefaults "github.com/rancher/rancher/tests/v2/validation/pipeline/qase"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/qase/testcase"
	"github.com/sirupsen/logrus"
	qase "go.qase.io/client"
	"gopkg.in/yaml.v2"
)

const (
	automationSuiteID    = int32(14)
	failStatus           = "fail"
	passStatus           = "pass"
	automationTestNameID = 15
	testSourceID         = 14
	testSource           = "GoValidation"
	multiSubTestPattern  = `(\w+/\w+/\w+){1,}`
	subtestPattern       = `(\w+/\w+){1,1}`
	testResultsJSON      = "results.json"
)

var (
	multiSubTestReg = regexp.MustCompile(multiSubTestPattern)
	subTestReg      = regexp.MustCompile(subtestPattern)
	qaseToken       = os.Getenv(qasedefaults.QaseTokenEnvVar)
	runIDEnvVar     = os.Getenv(qasedefaults.TestRunEnvVar)
)

func main() {
	if runIDEnvVar != "" {
		cfg := qase.NewConfiguration()
		cfg.AddDefaultHeader("Token", qaseToken)
		client := qase.NewAPIClient(cfg)

		runID, err := strconv.ParseInt(runIDEnvVar, 10, 64)
		if err != nil {
			logrus.Fatalf("error reporting converting string to int64: %v", err)
		}

		err = reportTestQases(client, runID)
		if err != nil {
			logrus.Error("error reporting: ", err)
		}
	}

}

func getAllAutomationTestCases(client *qase.APIClient) (map[string]qase.TestCase, error) {
	testCases := []qase.TestCase{}
	testCaseNameMap := map[string]qase.TestCase{}
	var numOfTestsCases int32 = 1
	offSetCount := 0
	for numOfTestsCases > 0 {
		offset := optional.NewInt32(int32(offSetCount))
		localVarOptionals := &qase.CasesApiGetCasesOpts{
			Offset: offset,
		}
		tempResult, _, err := client.CasesApi.GetCases(context.TODO(), qasedefaults.RancherManagerProjectID, localVarOptionals)
		if err != nil {
			return nil, err
		}

		testCases = append(testCases, tempResult.Result.Entities...)
		numOfTestsCases = tempResult.Result.Count
		offSetCount += 10
	}

	for _, testCase := range testCases {
		automationTestNameCustomField := getAutomationTestName(testCase.CustomFields)
		if automationTestNameCustomField != "" {
			testCaseNameMap[automationTestNameCustomField] = testCase
		} else {
			testCaseNameMap[testCase.Title] = testCase
		}

	}

	return testCaseNameMap, nil
}

func readTestCase() ([]testcase.GoTestOutput, error) {
	file, err := os.Open(testResultsJSON)
	if err != nil {
		return nil, err
	}

	fscanner := bufio.NewScanner(file)
	testCases := []testcase.GoTestOutput{}
	for fscanner.Scan() {
		var testCase testcase.GoTestOutput
		err = yaml.Unmarshal(fscanner.Bytes(), &testCase)
		if err != nil {
			return nil, err
		}
		testCases = append(testCases, testCase)
	}

	return testCases, nil
}

func parseCorrectTestCases(testCases []testcase.GoTestOutput) map[string]*testcase.GoTestCase {
	finalTestCases := map[string]*testcase.GoTestCase{}
	var deletedTest string
	for _, testCase := range testCases {
		if testCase.Action == "run" && strings.Contains(testCase.Test, "/") {
			newTestCase := &testcase.GoTestCase{Name: testCase.Test}
			finalTestCases[testCase.Test] = newTestCase
		} else if testCase.Action == "output" && strings.Contains(testCase.Test, "/") {
			goTestCase := finalTestCases[testCase.Test]
			goTestCase.StackTrace += testCase.Output
		} else if (testCase.Action == failStatus || testCase.Action == passStatus) && strings.Contains(testCase.Test, "/") {
			goTestCase := finalTestCases[testCase.Test]

			if goTestCase != nil {
				substring := subTestReg.FindString(goTestCase.Name)
				goTestCase.StackTrace += testCase.Output
				goTestCase.Status = testCase.Action
				goTestCase.Elapsed = testCase.Elapsed

				if multiSubTestReg.MatchString(goTestCase.Name) && substring != deletedTest {
					deletedTest = subTestReg.FindString(goTestCase.Name)
					delete(finalTestCases, deletedTest)
				}

			}
		}
	}

	return finalTestCases
}

func reportTestQases(client *qase.APIClient, testRunID int64) error {
	tempTestCases, err := readTestCase()
	if err != nil {
		return nil
	}

	goTestCases := parseCorrectTestCases(tempTestCases)

	qaseTestCases, err := getAllAutomationTestCases(client)
	if err != nil {
		return err
	}

	for _, goTestCase := range goTestCases {
		if testQase, ok := qaseTestCases[goTestCase.Name]; ok {
			// update test status
			err = updateTestInRun(client, *goTestCase, testQase.Id, testRunID)
			if err != nil {
				return err
			}
		} else {
			// write test case
			caseID, err := writeTestCaseToQase(client, *goTestCase)
			if err != nil {
				return err
			}
			err = updateTestInRun(client, *goTestCase, caseID.Result.Id, testRunID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func writeTestCaseToQase(client *qase.APIClient, testCase testcase.GoTestCase) (*qase.IdResponse, error) {
	testQaseBody := qase.TestCaseCreate{
		Title:      testCase.Name,
		SuiteId:    int64(automationSuiteID),
		IsFlaky:    int32(0),
		Automation: int32(2),
		CustomField: map[string]string{
			fmt.Sprintf("%d", testSourceID): testSource,
		},
	}
	caseID, _, err := client.CasesApi.CreateCase(context.TODO(), testQaseBody, qasedefaults.RancherManagerProjectID)
	if err != nil {
		return nil, err
	}
	return &caseID, err
}

func updateTestInRun(client *qase.APIClient, testCase testcase.GoTestCase, qaseTestCaseID, testRunID int64) error {
	status := fmt.Sprintf("%sed", testCase.Status)
	elasedTime, err := strconv.ParseFloat(testCase.Elapsed, 64)
	if err != nil {
		return err
	}

	resultBody := qase.ResultCreate{
		CaseId:  qaseTestCaseID,
		Status:  status,
		Comment: testCase.StackTrace,
		Time:    int64(elasedTime),
	}

	_, _, err = client.ResultsApi.CreateResult(context.TODO(), resultBody, qasedefaults.RancherManagerProjectID, testRunID)
	if err != nil {
		return err
	}

	return nil
}

func getAutomationTestName(customFields []qase.CustomFieldValue) string {
	for _, field := range customFields {
		if field.Id == automationTestNameID {
			return field.Value
		}
	}
	return ""
}
