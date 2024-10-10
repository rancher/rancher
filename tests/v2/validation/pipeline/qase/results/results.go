package results

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// ReportTest is a function that reports the test results.
func ReportTest() error {
	runID := os.Getenv("QASE_TEST_RUN_ID")
	if runID == "" {
		logrus.Error("QASE_TEST_RUN_ID is not set")
		return nil
	}

	user, err := user.Current()
	if err != nil {
		return err
	}

	reporterPath := filepath.Join(user.HomeDir, "go/src/github.com/rancher/rancher/tests/v2/validation/pipeline/scripts/build_qase_reporter.sh")

	cmd := exec.Command(reporterPath)
	output, err := cmd.Output()
	if err != nil {
		logrus.Error("Error running reporter script: ", err)
		return err
	}

	logrus.Info(string(output))

	return nil
}
