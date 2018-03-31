package clusteryaml

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
)

type Builder struct {
	clusters   v3.ClusterInterface
	nodeLister v3.NodeLister
	secrets    typedv1.SecretsGetter
	dialer     dialer.Factory

	dockerDialer hosts.DialerFactory
	localDialer  hosts.DialerFactory
}

func NewBuilder(dialer dialer.Factory, nodes v3.NodeLister, secrets typedv1.SecretsGetter) *Builder {
	local := &rkedialerfactory.RKEDialerFactory{
		Factory: dialer,
	}

	docker := &rkedialerfactory.RKEDialerFactory{
		Factory: dialer,
		Docker:  true,
	}

	return &Builder{
		nodeLister:   nodes,
		secrets:      secrets,
		dialer:       dialer,
		dockerDialer: docker.Build,
		localDialer:  local.Build,
	}
}

func (r *Builder) wrapTransport(clusterName string) (k8s.WrapTransport, error) {
	dialer, err := r.dialer.ClusterDialer(clusterName)
	if err != nil {
		return nil, err
	}

	return func(rt http.RoundTripper) http.RoundTripper {
		if ht, ok := rt.(*http.Transport); ok {
			ht.Dial = dialer
		}
		return rt
	}, nil
}

func (r *Builder) RESTConfig(bundle *rkecerts.Bundle, cluster *v3.Cluster) (*rest.Config, error) {
	newSpec, err := r.GetSpec(cluster, false)
	if err != nil {
		return nil, err
	}

	rkeCluster, err := r.ParseCluster(cluster.Name, newSpec)
	if err != nil {
		return nil, err
	}

	wt, err := r.wrapTransport(cluster.Name)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host: fmt.Sprintf("https://%s:443", rkeCluster.KubernetesServiceIP.String()),
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   cert.EncodeCertPEM(bundle.Certs()[pki.CACertName].Certificate),
			KeyData:  cert.EncodePrivateKeyPEM(bundle.Certs()[pki.KubeAdminCertName].Key),
			CertData: cert.EncodeCertPEM(bundle.Certs()[pki.KubeAdminCertName].Certificate),
		},
		WrapTransport: wt,
	}, nil
}

func (r *Builder) GetSpec(cluster *v3.Cluster, etcdOnly bool) (*v3.ClusterSpec, error) {
	allNodes, err := r.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	nodes, err := rkeNodes(allNodes, etcdOnly)
	if err != nil {
		return nil, err
	}

	var config v3.RancherKubernetesEngineConfig
	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		config = *cluster.Spec.RancherKubernetesEngineConfig
	}

	images, err := getSystemImages(cluster.Spec)
	if err != nil {
		return nil, err
	}

	config.Nodes = nodes
	config.SystemImages = *images

	spec := cluster.Spec
	spec.RancherKubernetesEngineConfig = &config

	return &spec, nil
}

func (r *Builder) GetOrGenerateWithNode(cluster *v3.Cluster, node *v3.RKEConfigNode) (*rkecerts.Bundle, error) {
	rkeCluster, bundle, err := r.getOrGenerateCerts(cluster)
	if err != nil {
		return nil, err
	}

	if !slice.ContainsString(node.Role, services.ETCDRole) {
		return bundle, nil
	}

	_, nodeName := ref.Parse(node.NodeName)
	var nodeBundle *rkecerts.Bundle
	store := encryptedstore.NewNamespacedGenericEncrypedStore("node-cert-", cluster.Name, r.secrets)
	value, err := store.Get(nodeName)
	if errors.IsNotFound(err) {
		nodeBundle, err = r.generateAndSave(store, bundle, nodeName, rkeCluster)
	} else if err != nil {
		return nil, err
	} else {
		nodeBundle, err = rkecerts.Unmarshal(value["certs"])
	}

	if err != nil {
		return nil, err
	}

	bundle.Merge(nodeBundle)
	return bundle, nil
}

func (r *Builder) generateAndSave(store *encryptedstore.GenericEncryptedStore, bundle *rkecerts.Bundle, nodeName string, rkeCluster *cluster.Cluster) (*rkecerts.Bundle, error) {
	nodeBundle, err := bundle.GenerateETCD(nodeName, rkeCluster)
	if err != nil {
		return nil, err
	}

	content, err := nodeBundle.Marshal()
	if err != nil {
		return nil, err
	}

	return nodeBundle, store.Set(nodeName, map[string]string{
		"certs": content,
	})
}

func (r *Builder) GetOrGenerateCerts(cluster *v3.Cluster) (*rkecerts.Bundle, error) {
	_, b, err := r.getOrGenerateCerts(cluster)
	return b, err
}

func (r *Builder) getOrGenerateCerts(cluster *v3.Cluster) (*cluster.Cluster, *rkecerts.Bundle, error) {
	spec, err := r.GetSpec(cluster, false)
	if err != nil {
		return nil, nil, err
	}

	rkeCluster, err := librke.New().ParseCluster(cluster.Name, spec.RancherKubernetesEngineConfig, nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	store := encryptedstore.NewNamespacedGenericEncrypedStore("cluster-cert-", cluster.Name, r.secrets)
	value, err := store.Get(cluster.Name)
	if err == nil {
		b, err := rkecerts.Unmarshal(value["certs"])
		return rkeCluster, b, err
	} else if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	bundle, err := rkecerts.Generate(&rkeCluster.RancherKubernetesEngineConfig)
	if err != nil {
		return nil, nil, err
	}

	certs, err := bundle.Marshal()
	if err != nil {
		return nil, nil, err
	}

	err = store.Set(cluster.Name, map[string]string{
		"certs": certs,
	})
	if err != nil {
		return nil, nil, err
	}

	return rkeCluster, bundle, nil
}

func (r *Builder) ParseCluster(clusterName string, spec *v3.ClusterSpec) (*cluster.Cluster, error) {
	if spec == nil || len(spec.RancherKubernetesEngineConfig.Nodes) == 0 {
		return nil, nil
	}

	wt, err := r.wrapTransport(clusterName)
	if err != nil {
		return nil, err
	}
	return librke.New().ParseCluster(clusterName, spec.RancherKubernetesEngineConfig, r.dockerDialer, r.localDialer, wt)
}

func rkeNodes(nodes []*v3.Node, etcdOnly bool) ([]v3.RKEConfigNode, error) {
	var rkeConfigNodes []v3.RKEConfigNode
	for _, machine := range nodes {
		if machine.DeletionTimestamp != nil {
			continue
		}

		if etcdOnly && v3.NodeConditionProvisioned.IsUnknown(machine) {
			return nil, &controller.ForgetError{
				Err: fmt.Errorf("waiting for %s to finish provisioning", machine.Spec.RequestedHostname),
			}
		}

		if machine.Status.NodeConfig == nil {
			continue
		}

		if len(machine.Status.NodeConfig.Role) == 0 {
			continue
		}

		if etcdOnly && !slice.ContainsString(machine.Status.NodeConfig.Role, "etcd") {
			continue
		}

		if !v3.NodeConditionProvisioned.IsTrue(machine) {
			continue
		}

		node := *machine.Status.NodeConfig
		if node.User == "" {
			node.User = "root"
		}
		if len(node.Role) == 0 {
			node.Role = []string{"worker"}
		}
		if node.Port == "" {
			node.Port = "22"
		}
		if node.NodeName == "" {
			node.NodeName = fmt.Sprintf("%s:%s", machine.Namespace, machine.Name)
		}
		rkeConfigNodes = append(rkeConfigNodes, node)
	}

	sort.Slice(rkeConfigNodes, func(i, j int) bool {
		return rkeConfigNodes[i].NodeName < rkeConfigNodes[j].NodeName
	})

	return rkeConfigNodes, nil
}

func getSystemImages(spec v3.ClusterSpec) (*v3.RKESystemImages, error) {
	// fetch system images from settings
	systemImagesStr := settings.KubernetesVersionToSystemImages.Get()
	if systemImagesStr == "" {
		return nil, fmt.Errorf("Failed to load setting %s", settings.KubernetesVersionToSystemImages.Name)
	}
	systemImagesMap := make(map[string]v3.RKESystemImages)
	if err := json.Unmarshal([]byte(systemImagesStr), &systemImagesMap); err != nil {
		return nil, err
	}

	version := spec.RancherKubernetesEngineConfig.Version
	if version == "" {
		version = settings.KubernetesVersion.Get()
	}

	systemImages, ok := systemImagesMap[version]
	if !ok {
		return nil, fmt.Errorf("Failed to find system images for version %v", version)
	}

	if len(spec.RancherKubernetesEngineConfig.PrivateRegistries) == 0 {
		return &systemImages, nil
	}

	// prepend private repo
	privateRegistry := spec.RancherKubernetesEngineConfig.PrivateRegistries[0]
	imagesMap, err := convert.EncodeToMap(systemImages)
	if err != nil {
		return nil, err
	}
	updatedMap := make(map[string]interface{})
	for key, value := range imagesMap {
		newValue := fmt.Sprintf("%s/%s", privateRegistry.URL, value)
		updatedMap[key] = newValue
	}
	if err := mapstructure.Decode(updatedMap, &systemImages); err != nil {
		return nil, err
	}
	return &systemImages, nil
}
