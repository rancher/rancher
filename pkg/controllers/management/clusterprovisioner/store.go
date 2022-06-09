package clusterprovisioner

import (
	"encoding/json"

	"github.com/rancher/rancher/pkg/encryptedstore"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	"github.com/sirupsen/logrus"
)

const (
	dataKey = "cluster"
)

func NewPersistentStore(namespaces v1.NamespaceInterface, secretsGetter v1.SecretsGetter) cluster.PersistentStore {
	store, err := encryptedstore.NewGenericEncryptedStore("c-", "", namespaces, secretsGetter)
	if err != nil {
		logrus.Fatal(err)
	}

	return &engineStore{
		store: store,
	}
}

type engineStore struct {
	store *encryptedstore.GenericEncryptedStore
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
	return s.store.Set(cluster.Name, map[string]string{
		dataKey: string(content),
	})
}

func (s *engineStore) PersistStatus(cluster cluster.Cluster, status string) error {
	cluster.Status = status
	return s.Store(cluster)
}
