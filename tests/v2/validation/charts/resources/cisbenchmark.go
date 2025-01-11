package resources

import (
	"fmt"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	cis "github.com/rancher/rancher/tests/v2/validation/provisioning/resources/cisbenchmark"
	"github.com/rancher/shepherd/clients/rancher"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CisConfigFileKey        = "cisConfig"
	CISBenchmarkProjectName = "cis-operator-system"
)

type CisConfig struct {
	ProfileName  string `json:"profileName" yaml:"profileName"`
	ChartVersion string `json:"chartVersion" yaml:"chartVersion"`
}

func VerifyCISChartInstallation(client *rancher.Client, clusterID string, chartVersion string, cisProfileName string) error {
	// Upgrade the CIS benchmark chart

	err := extencharts.WatchAndWaitDeployments(client, clusterID, charts.CISBenchmarkNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	err = extencharts.WatchAndWaitDaemonSets(client, clusterID, charts.CISBenchmarkNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}

	cisbenchmarkChartPostUpgrade, err := extencharts.GetChartStatus(client, clusterID, charts.CISBenchmarkNamespace, charts.CISBenchmarkName)
	if err != nil {
		return err
	}

	chartVersionPostUpgrade := cisbenchmarkChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	if chartVersion != chartVersionPostUpgrade {
		return fmt.Errorf("Expected chart version %s, got %s", chartVersion, chartVersionPostUpgrade)
	}

	// Run the scan
	err = cis.RunCISScan(client, clusterID, cisProfileName, true)
	if err != nil {
		return err
	}
	return nil
}
