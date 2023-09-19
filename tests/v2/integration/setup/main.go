//go:build integrationsetup

// We're protecting this file with a build tag because it depends on github.com/containers/image which depends on C
// libraries that we can't and don't want to build unless we're going to run this integration setup program.

package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/creasty/defaults"
	"github.com/davecgh/go-spew/spew"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rancherClient "github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	pkgpf "github.com/rancher/rancher/tests/framework/pkg/portforward"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	testdefaults "github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	clusterNameBaseName = "integration-test-cluster"
)

// main creates a test namespace and cluster for use in integration tests.
func main() {
	// Make sure a valid cluster agent image tag was provided before doing anything else. The envvar CATTLE_AGENT_IMAGE
	// should be the image name (and tag) assigned to the cattle cluster agent image that was just built during CI.
	agentImage := os.Getenv("CATTLE_AGENT_IMAGE")
	if agentImage == "" {
		logrus.Fatal("Envvar CATTLE_AGENT_IMAGE must be set to a valid rancher-agent Docker image")
	}

	logrus.Infof("Generating test config")
	ipAddress, err := getOutboundIP()
	if err != nil {
		logrus.Fatalf("Error getting outbound IP address: %v", err)
	}

	hostURL := fmt.Sprintf("%s:8443", ipAddress.String())

	var userToken *management.Token

	err = kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		userToken, err = token.GenerateUserToken(&management.User{
			Username: "admin",
			Password: "admin",
		}, hostURL)
		if err != nil {
			return false, nil
		}

		return true, nil
	})

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
	fmt.Println("FELIPE --- Sleeping 10s after clusterClient")
	time.Sleep(10 * time.Second)

	logrus.Info("Creating test namespace")
	ns, err := namespace.Random(clusterClients)
	if err != nil {
		logrus.Fatalf("Error creating namespace: %v", err)
	}

	logrus.Infof("Deploying registry to default namespace with secrets in namespace %s", ns.Name)
	reg, err := registry.CreateOrGetRegistry(clusterClients, ns.Name, "registry", false)
	if err != nil {
		logrus.Fatalf("Error creating registry: %v", err)
	}

	logrus.Infof("Deploying registry-cache to default namespace with secrets in namespace %s", ns.Name)
	regCache, err := registry.CreateOrGetRegistry(clusterClients, ns.Name, "registry-cache", true)
	if err != nil {
		logrus.Fatalf("Error creating registry-cache: %v", err)
	}

	{
		// Merge the two registry configs so that when we look for an image in the downstream cluster, we will:
		//  - First check "registry" (where we push our recently-built test images)
		//  - If not found "registry", check "registry-cache" (i.e. the pull-through cache)
		//  - If not present in "registry-cache", it will pull the image from docker.io
		mirrors := reg.Mirrors["docker.io"]
		mirrors.Endpoints = append(mirrors.Endpoints, regCache.Mirrors["docker.io"].Endpoints...)
		reg.Mirrors["docker.io"] = mirrors
		for k, v := range regCache.Configs {
			reg.Configs[k] = v
		}
	}

	// Set up a port-forward to the registry pod, so we can copy images to it.
	stopCh := make(chan struct{}, 1)
	errCh := make(chan error)
	defer func() {
		// This will stop the port-forward if it's still running
		close(stopCh)

		// Print any errors returned by the port-forwarder
		select {
		case err := <-errCh:
			logrus.Errorf("error in port-forward: %v", err)
		default: // Do nothing
		}
		close(errCh)
	}()

	logrus.Info("Forwarding local port 5000 to registry:5000")
	if err = pkgpf.ForwardPorts(
		clusterClients.RESTConfig,
		"default",
		"registry",
		[]string{"5000:5000"},
		stopCh,
		errCh,
		time.Second*10,
	); err != nil {
		logrus.Fatalf("Error forwarding ports: %v", err)
	}

	// Copy the local image rancher/rancher-agent:master-head from the Docker daemon to the Docker registry
	// at localhost:5000 where it will be given the tag "test-local". We need to do this before we create downstream
	// clusters to ensure that they can use this locally-built image. We're also retrying on error here because the
	// port-forward tends to be flaky when we're transferring large images, at least on my machine.
	shouldRetry := func(err error) bool {
		if err != nil {
			logrus.Errorf("Error pushing images: %v", err)
		}

		return err != nil
	}
	attemptImagePush := func() error {
		logrus.Info("Attempting to push images to registry")
		return pushImages(map[string]string{
			"docker-daemon:" + agentImage: "docker://localhost:5000/" + agentImage,
		})
	}
	if err = retry.OnError(wait.Backoff{Steps: 10}, shouldRetry, attemptImagePush); err != nil {
		logrus.Fatalf("Failed to push images to registry: %v", err)
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

	fmt.Println("FELIPE --- Sleeping 20s after clusterCreate")
	time.Sleep(20 * time.Second)

	logrus.WithField("dump", func() string {
		r := bytes.Buffer{}
		spew.Fdump(&r, c)
		return r.String()
	}()).Errorf("FELIPE-DEBUG-DUMP")
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

// pushImages does the equivalent of
// "skopeo copy --dest-tls-verify=false --dest-creds=admin:admin <src> <dst>"
// for each image. imageMapping should map source image to destination image in skopeo format.
func pushImages(imageMapping map[string]string) error {
	// Don't bother verifying any image signatures
	policyCtx, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	})
	if err != nil {
		return fmt.Errorf("error creating policy context: %v", err)
	}

	for src, dst := range imageMapping {
		srcRef, err := alltransports.ParseImageName(src)
		if err != nil {
			return fmt.Errorf("error parsing source image %s: %v", src, err)
		}

		dstRef, err := alltransports.ParseImageName(dst)
		if err != nil {
			return fmt.Errorf("error parsing destination image %s: %v", dst, err)
		}

		if _, err = copy.Image(
			context.Background(),
			policyCtx,
			dstRef,
			srcRef,
			&copy.Options{
				DestinationCtx: &types.SystemContext{
					// Don't use TLS because the registry we're pushing to uses self-signed certs.
					DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
					// Use registry credentials admin:admin
					DockerAuthConfig: &types.DockerAuthConfig{
						Username: "admin",
						Password: "admin",
					},
				},
			},
		); err != nil {
			return fmt.Errorf("error copying image from source %s to destination %s: %v", src, dst, err)
		}
	}

	return nil
}
