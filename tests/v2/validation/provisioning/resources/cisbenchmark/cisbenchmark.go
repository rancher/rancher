package charts

import (
	"time"

	cis "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/shepherd/clients/rancher"
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

	cisBenchmarkSteveType = "cis.cattle.io.clusterscan"
)

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
func RunCISScan(client *rancher.Client, projectClusterID, scanProfileName string) error {
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

	return nil
}
