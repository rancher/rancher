package rkenodeconfigserver

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	kecerts "github.com/rancher/rancher/pkg/kontainer-engine/drivers/rke/rkecerts"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rke/pki"
)

type BundleLookup struct {
	engineStore cluster.PersistentStore
}

func NewLookup(namespaces v1.NamespaceInterface, secrets v1.SecretsGetter) *BundleLookup {
	return &BundleLookup{
		engineStore: clusterprovisioner.NewPersistentStore(namespaces, secrets),
	}
}

func (r *BundleLookup) Lookup(cluster *v3.Cluster) (*rkecerts.Bundle, error) {
	c, err := r.engineStore.Get(cluster.Name)
	if err != nil {
		return nil, err
	}

	certs, ok := c.Metadata["Certs"]
	if !ok {
		return nil, fmt.Errorf("waiting for certs to be generated for cluster %s", cluster.Name)
	}

	certMap, err := kecerts.LoadString(certs)
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

	return rkecerts.NewBundle(newCertMap), nil
}
