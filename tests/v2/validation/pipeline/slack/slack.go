package slack

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/qase/testcase"
)

var (
	slackWebhook = os.Getenv("SLACK_WEBHOOK")
)

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Block struct {
	Type    string `json:"type"`
	Text    Text   `json:"text"`
	BlockID string `json:"block_id"`
}

func setupTestSlackBlocks(testCaseSlice []*testcase.GoTestCase, runID int64, testRunName string) []Block {
	var testSuite string
	var blockSlice []Block

	titleBlock := Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: fmt.Sprintf("Failures: %s", testRunName),
		},
	}

	blockSlice = append(blockSlice, titleBlock)
	for _, testCase := range testCaseSlice {
		combinedTestSuite := strings.Join(testCase.TestSuite, "/")
		if combinedTestSuite != testSuite {
			testSuite = combinedTestSuite
			testSuiteBlock := Block{
				Type: "section",
				Text: Text{
					Type: "mrkdwn",
					Text: fmt.Sprintf("TestSuite: %s", testSuite),
				},
			}
			blockSlice = append(blockSlice, testSuiteBlock)
		}

		testCaseBlock := Block{
			Type: "section",
			Text: Text{
				Type: "mrkdwn",
				Text: fmt.Sprintf("<https://app.qase.io/run/RM/dashboard/%v|%s>:Failed\n", runID, testCase.Name),
			},
		}
		blockSlice = append(blockSlice, testCaseBlock)

	}
	return blockSlice
}

// PostSlackMesasge is a function that posts the end to end validation results to our specified slack channel
func PostSlackMessage(testCaseSlice []*testcase.GoTestCase, runID int64, testRunName string) error {
	slackBlocks := setupTestSlackBlocks(testCaseSlice, runID, testRunName)

	bodyContent, err := json.Marshal(struct {
		Text   string  `json:"text"`
		Blocks []Block `json:"blocks"`
	}{
		Text:   "Recurring Runs Failures",
		Blocks: slackBlocks,
	})

	if err != nil {
		return fmt.Errorf("error with marshal slack message: %v", err)
	}

	err = postAction(slackWebhook, bodyContent)
	if err != nil {
		return fmt.Errorf("error with slack message post: %v", err)
	}

	return nil
}

func postAction(url string, body []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return httperror.NewAPIErrorLong(resp.StatusCode, resp.Status, url)
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}
