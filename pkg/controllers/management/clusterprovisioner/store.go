package clusterprovisioner

import (
	"encoding/json"
	"hash/fnv"

	"github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/encryptedstore"
	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
)

const (
	dataKey = "cluster"
)

func NewPersistentStore(namespaces v1.NamespaceInterface, secretsGetter v1.SecretsGetter) cluster.PersistentStore {
	store, err := encryptedstore.NewGenericEncrypedStore("c-", "", namespaces, secretsGetter)
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

func (s *engineStore) Store(cls cluster.Cluster) error {
	content, err := json.Marshal(cls)
	if err != nil {
		return err
	}
	h := fnv.New32a()
	hash, err := h.Write(content)
	if err != nil {
		logrus.Errorf("RAJASHREE_PRO_STORE [Store]: Error hashing cluster %v: %v", cls.Name, err)
	}
	logrus.Infof("RAJASHREE_PRO_STORE [Store]: Marshaled content for cluster %v: %v", cls.Name, hash)
	err = s.store.Set(cls.Name, map[string]string{
		dataKey: string(content),
	})
	if err != nil {
		logrus.Errorf("RAJASHREE_PRO_STORE [Store]: Error storing cluster %v: %v", cls.Name, err)
	}
	// Debug: checking if state saved
	var checkCluster cluster.Cluster
	data, err := s.store.Get(cls.Name)
	if err != nil {
		logrus.Errorf("RAJASHREE_PRO_STORE [Store]: Error getting cluster %v: %v", cls.Name, err)
	}
	err = json.Unmarshal([]byte(data[dataKey]), &checkCluster)
	if err != nil {
		logrus.Errorf("RAJASHREE_PRO_STORE [Store]: Error unmarshaling cluster %v: %v", cls.Name, err)
		return nil
	}
	logrus.Infof("RAJASHREE_PRO_STORE [Store]: Stored cluster state for %v: %v", cls.Name, checkCluster.Status)
	return nil
}

func (s *engineStore) PersistStatus(cluster cluster.Cluster, status string) error {
	cluster.Status = status
	logrus.Infof("RAJASHREE_PRO_STORE [PersistStatus]: Storing cluster %v in status %v", cluster.Name, status)
	return s.Store(cluster)
}
