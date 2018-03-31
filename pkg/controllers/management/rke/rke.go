package rke

import (
	"context"
	"fmt"
	"time"

	"encoding/base64"

	"reflect"

	"github.com/rancher/kontainer-engine/drivers"
	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/clusteryaml"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/flowcontrol"
)

type Provisioner struct {
	Clusters    v3.ClusterInterface
	EventLogger event.Logger
	backOff     *flowcontrol.Backoff
	builder     *clusteryaml.Builder
}

func New(management *config.ManagementContext, driver service.EngineService) *Provisioner {
	return &Provisioner{
		Clusters:    management.Management.Clusters(""),
		EventLogger: management.EventLogger,
		backOff:     flowcontrol.NewBackOff(30*time.Second, 10*time.Minute),
		builder: clusteryaml.NewBuilder(management.Dialer,
			management.Management.Nodes("").Controller().Lister(),
			management.K8sClient.CoreV1()),
	}
}

func (p *Provisioner) Prepare(cluster *v3.Cluster) (*v3.Cluster, error) {
	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		spec, err := p.GetSpec(cluster)
		if err != nil {
			return cluster, err
		}
		return cluster, p.isReady(spec)
	}

	_, err := v3.ClusterConditionCertsGenerated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		_, err := p.builder.GetOrGenerateCerts(cluster)
		return cluster, err
	})
	if err != nil {
		return cluster, &controller.ForgetError{
			Err: err,
		}
	}

	return cluster, nil
}

func (p *Provisioner) isReady(spec *v3.ClusterSpec) error {
	var (
		etcd, controlPlane bool
	)

	for _, node := range spec.RancherKubernetesEngineConfig.Nodes {
		if slice.ContainsString(node.Role, "etcd") {
			etcd = true
		}
		if slice.ContainsString(node.Role, "controlplane") {
			controlPlane = true
		}
	}

	if !etcd || !controlPlane {
		return &controller.ForgetError{
			Err: fmt.Errorf("waiting for etcd and controlplane nodes to be registered"),
		}
	}

	return nil
}

func (p *Provisioner) GetSpec(cluster *v3.Cluster) (*v3.ClusterSpec, error) {
	return p.builder.GetSpec(cluster, false)
}

func (p *Provisioner) Remove(ctx context.Context, cluster *v3.Cluster) error {
	return nil
}

func (p *Provisioner) Provision(ctx context.Context, cluster *v3.Cluster, update bool) (string, string, string, error) {
	orig := cluster.DeepCopy()
	obj, err := v3.ClusterConditionEtcd.Do(cluster, func() (runtime.Object, error) {
		return p.etcd(cluster)
	})
	if !reflect.DeepEqual(orig, obj) {
		obj, _ = p.Clusters.Update(obj.(*v3.Cluster))
	}
	if err != nil {
		return "", "", "", err
	}
	cluster = obj.(*v3.Cluster)

	bundle, err := p.builder.GetOrGenerateCerts(cluster)
	if err != nil {
		return "", "", "", err
	}

	if cluster.Status.ServiceAccountToken != "" {
		return cluster.Status.APIEndpoint, cluster.Status.ServiceAccountToken, cluster.Status.CACert, nil
	}

	restConfig, err := p.builder.RESTConfig(bundle, cluster)
	if err != nil {
		return "", "", "", err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", "", "", err
	}

	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return "", "", "", fmt.Errorf("waiting for cluster agent: %v", err)
	}

	token, err := drivers.GenerateServiceAccountToken(client)
	if err != nil {
		return "", "", "", err
	}

	caCert := base64.StdEncoding.EncodeToString(cert.EncodeCertPEM(bundle.Certs()[pki.CACertName].Certificate))
	return restConfig.Host, token, caCert, nil
}
