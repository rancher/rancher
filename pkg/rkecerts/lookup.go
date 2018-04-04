package rkecerts

import (
	"fmt"

	"github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/kontainer-engine/drivers/rke/rkecerts"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type BundleLookup struct {
	engineStore cluster.PersistentStore
}

func NewLookup(namespaces v1.NamespaceInterface, secrets typedv1.SecretsGetter) *BundleLookup {
	return &BundleLookup{
		engineStore: clusterprovisioner.NewPersistentStore(namespaces, secrets),
	}
}

func (r *BundleLookup) Lookup(cluster *v3.Cluster) (*Bundle, error) {
	c, err := r.engineStore.Get(cluster.Name)
	if err != nil {
		return nil, err
	}

	certs, ok := c.Metadata["Certs"]
	if !ok {
		return nil, fmt.Errorf("waiting for certs to be generated for cluster %s", cluster.Name)
	}

	certMap, err := rkecerts.LoadString(certs)
	if err != nil {
		return nil, err
	}

	newCertMap := map[string]pki.CertificatePKI{}
	for k, v := range certMap {
		if v.Config != "" {
			v.ConfigPath = pki.GetConfigPath(k)
		}
		if v.Key != nil {
			v.KeyPath = pki.GetKeyPath(k)
		}
		if v.Certificate != nil {
			v.Path = pki.GetCertPath(k)
		}
		newCertMap[k] = v
	}

	return newBundle(newCertMap), nil
}
