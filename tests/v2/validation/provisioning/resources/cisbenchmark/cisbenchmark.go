package charts

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	cis "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/defaults"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	System = "System"
	pass   = "pass"
	scan   = "scan"
	fail   = "fail"

	cisBenchmarkSteveType = "cis.cattle.io.clusterscan"
	clusterScanReportType = "cis.cattle.io.clusterscanreport"
)

// CisReport is the report structure stored as report json in cluster scan report spec.
type CisReport struct {
	Total         int
	Pass          int
	Fail          int
	Skip          int
	Warn          int
	NotApplicable int
	Results       []*Group `json:"results"`
}

// Group is the result structure stored as report json in Results of CisReport
type Group struct {
	ID     string      `yaml:"id" json:"id"`
	Text   string      `json:"description"`
	Checks []*CisCheck `json:"checks"`
}

// CisCheck is the ID, Description and State structure of individual test in cluster scan.
type CisCheck struct {
	Id          string
	Description string
	State       string
}

// SetupCISBenchmarkChart installs the CIS Benchmark chart and waits for all resources to be ready.
func SetupCISBenchmarkChart(client *rancher.Client, projectClusterID string, chartInstallOptions *charts.InstallOptions, benchmarkNamespace string) error {
	logrus.Infof("Installing CIS Benchmark chart...")
	err := charts.InstallCISBenchmarkChart(client, chartInstallOptions)
	if err != nil {
		return err
	}

	logrus.Infof("Waiting for CIS Benchmark chart deployments to have expected number of available replicas...")
	err = extensionscharts.WatchAndWaitDeployments(client, projectClusterID, benchmarkNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	logrus.Infof("Waiting for CIS Benchmark chart DaemonSets to have expected number of available nodes...")
	err = extensionscharts.WatchAndWaitDaemonSets(client, projectClusterID, benchmarkNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	logrus.Infof("Waiting for CIS Benchmark chart StatefulSets to have expected number of ready replicas...")
	err = extensionscharts.WatchAndWaitStatefulSets(client, projectClusterID, benchmarkNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	logrus.Infof("Successfully installed CIS Benchmark chart!")

	return nil
}

// RunCISScan runs the CIS Benchmark scan with the specified profile name.
func RunCISScan(client *rancher.Client, projectClusterID, scanProfileName string, printFailedChecks bool) error {
	logrus.Infof("Running CIS Benchmark scan: %s", scanProfileName)

	cisScan := cis.ClusterScan{
		ObjectMeta: metav1.ObjectMeta{
			Name: namegen.AppendRandomString(scan),
		},
		Spec: cis.ClusterScanSpec{
			ScanProfileName: scanProfileName,
			ScoreWarning:    pass,
		},
	}

	steveclient, err := client.Steve.ProxyDownstream(projectClusterID)
	if err != nil {
		return err
	}

	scan, err := steveclient.SteveType(cisBenchmarkSteveType).Create(cisScan)
	if err != nil {
		return err
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 1*time.Second, defaults.TenMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		scanResp, err := steveclient.SteveType(cisBenchmarkSteveType).ByID(scan.ID)
		if err != nil {
			return false, err
		}

		if !scanResp.ObjectMeta.State.Transitioning {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	logrus.Infof("CIS Benchmark scan passed!")

	if printFailedChecks {
		logrus.Infof("Checking cluster scan report for failed checks")
		err = checkScanReport(steveclient, scan.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkScanReport(steveClient *v1.Client, scanName string) error {
	scanReportList, err := steveClient.SteveType(clusterScanReportType).List(nil)
	if err != nil {
		return err
	}

	scanReportIdx := -1

	for idx, scanReport := range scanReportList.Data {
		if strings.Contains(scanReport.Name, scanName) {
			scanReportIdx = idx
			break
		}
	}

	if scanReportIdx < 0 {
		return errors.New("scan report not found")
	}

	scanReport := &cis.ClusterScanReport{}
	scanReportSpec := scanReportList.Data[scanReportIdx]
	err = v1.ConvertToK8sType(scanReportSpec, scanReport)
	if err != nil {
		return err
	}

	reportData := &CisReport{}
	err = json.Unmarshal([]byte(scanReport.Spec.ReportJSON), &reportData)
	if err != nil {
		return err
	}

	logrus.Infof("Out of total number of %d scans, %d scans passed, %d scans were skipped and %d scans failed", reportData.Total, reportData.Pass, reportData.Skip, reportData.Fail)
	for _, group := range reportData.Results {
		for _, check := range group.Checks {
			if check.State == fail {
				logrus.Infof("check failed: id: %s state: %s description: %s ", check.Id, check.State, check.Description)
			}
		}
	}
	if reportData.Fail != 0 {
		return errors.New("cluster scan failed")
	}
	return nil
}
