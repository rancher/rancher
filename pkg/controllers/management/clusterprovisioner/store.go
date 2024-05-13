package clusterprovisioner

import (
	"encoding/json"

	"github.com/rancher/rancher/pkg/encryptedstore"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataKey = "cluster"
)

func NewPersistentStore(namespaces v1.NamespaceInterface, secretsGetter v1.SecretsGetter, clusterClient v3.ClusterInterface) cluster.PersistentStore {
	store, err := encryptedstore.NewGenericEncryptedStore("c-", "", namespaces, secretsGetter)
	if err != nil {
		logrus.Fatal(err)
	}

	return &engineStore{
		store:         store,
		clusterClient: clusterClient,
	}
}

type engineStore struct {
	store         *encryptedstore.GenericEncryptedStore
	clusterClient v3.ClusterInterface
}

func (s *engineStore) GetStatus(name string) (string, error) {
	cls, err := s.Get(name)
	if err != nil {
		return "", err
	}
	return cls.Status, nil
}

func (s *engineStore) Get(name string) (cluster.Cluster, error) {
	cluster := cluster.Cluster{}
	data, err := s.store.Get(name)
	if err != nil {
		return cluster, err
	}
	return cluster, json.Unmarshal([]byte(data[dataKey]), &cluster)
}

func (s *engineStore) Remove(name string) error {
	return s.store.Remove(name)
}

func (s *engineStore) Store(cluster cluster.Cluster) error {
	content, err := json.Marshal(cluster)
	if err != nil {
		return err
	}

	var owner *metav1.OwnerReference
	if s.clusterClient != nil {
		c, err := s.clusterClient.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		owner = &metav1.OwnerReference{
			APIVersion: c.APIVersion,
			Kind:       c.Kind,
			Name:       c.Name,
			UID:        c.UID,
		}
	}

	return s.store.Set(cluster.Name, map[string]string{
		dataKey: string(content),
	}, owner)
}

func (s *engineStore) PersistStatus(cluster cluster.Cluster, status string) error {
	cluster.Status = status
	return s.Store(cluster)
}
