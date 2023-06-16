package main

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/creasty/defaults"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rancherClient "github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	testdefaults "github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterNameBaseName  = "integration-test-cluster"
	clusterNamespaceBase = "test-ns"
)

// main creates a test namespace and cluster for use in integration tests.
func main() {
	logrus.Infof("Generating test config")
	ipAddress, err := getOutboundIP()
	if err != nil {
		logrus.Fatalf("Error getting outbound IP address: %v", err)
	}

	hostURL := fmt.Sprintf("%s:8443", ipAddress.String())
	userToken, err := token.GenerateUserToken(
		&management.User{
			Username: "admin",
			Password: "admin",
		},
		hostURL,
	)
	if err != nil {
		logrus.Fatalf("Error with generating admin token: %v", err)
	}

	cleanup := true
	rancherConfig := rancherClient.Config{
		AdminToken:  userToken.Token,
		Host:        hostURL,
		Cleanup:     &cleanup,
		ClusterName: namegen.AppendRandomString(clusterNameBaseName),
	}

	err = defaults.Set(&rancherConfig)
	if err != nil {
		logrus.Fatalf("Error with setting up config file: %v", err)
	}

	err = config.WriteConfig(rancherClient.ConfigurationFileKey, &rancherConfig)
	if err != nil {
		logrus.Fatalf("Error writing test config: %v", err)
	}

	// Note that we do not defer clusterClients.Close() here. This is because doing so would cause the test namespace
	// in which the downstream cluster resides to be deleted before it can be used in tests.
	clusterClients, err := clients.New()
	if err != nil {
		logrus.Fatalf("Error creating clients: %v", err)
	}

	logrus.Info("Creating test namespace")
	ns, err := namespace.Random(clusterClients)
	if err != nil {
		logrus.Fatalf("Error creating namespace: %v", err)
	}

	logrus.Info("Setting up registry cache in namespace default")
	reg, err := registry.GetCache(clusterClients, ns.Name)
	if err != nil {
		logrus.Fatalf("Error getting registry cache: %v", err)
	}

	// The purpose of this step is to ensure that our downstream cluster(s) use the images we just built during our
	// CI process rancher than the official ones from DockerHub. If we didn't do this, then we wouldn't be testing
	// against the code we just wrote – we'd be testing against code we released a while ago.
	// We may have to retry a few times to push images to the registry as the port-forward we're using can be flaky.
	maxAttempts := 10
	attempts := 0
	for ; attempts < maxAttempts; attempts++ {
		logrus.Infof("Pushing test images to registry")
		out, err := exec.Command("/usr/bin/bash", "tests/v2/integration/scripts/seed-registry").Output()
		if err != nil {
			logrus.Errorf("Pushing images to registry: %v. Process output:\n%s\nRetrying", err, string(out))
		} else {
			logrus.Infof("Successfully pushed test images")
			break
		}
	}

	if attempts >= maxAttempts {
		logrus.Fatalf("Failed to push images to registry after %d attempts", maxAttempts)
	}

	logrus.Infof(
		"Creating test cluster %s with %s in namespace %s",
		rancherConfig.ClusterName,
		testdefaults.SomeK8sVersion,
		ns.Name,
	)
	c, err := cluster.New(clusterClients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rancherConfig.ClusterName,
			Namespace: ns.Name,
		},
		Spec: provisioningv1api.ClusterSpec{
			KubernetesVersion: testdefaults.SomeK8sVersion,
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &testdefaults.One,
				}},
				RKEClusterSpecCommon: v1.RKEClusterSpecCommon{
					Registries: &reg,
				},
			},
		},
	})
	if err != nil {
		logrus.Fatalf("Error creating integration test cluster: %v", err)
	}

	logrus.Info("Waiting for test cluster to be ready")
	c, err = cluster.WaitForCreate(clusterClients, c)
	if err != nil {
		logrus.Fatalf("Error waiting for test cluster to be ready: %v", err)
	}

	logrus.Infof("Test cluster %s created successfully. Setup complete.", c.Name)
}

// Get preferred outbound ip of this machine
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}
