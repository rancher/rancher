package saml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	samlClientCacheNamespace = "cattle-system"
	samlClientCacheName      = "saml-client-ids"
)

var configMapLabels = map[string]string{
	"authz.management.cattle.io/provider": "saml",
}

var _ assertionStore = (*configMapAssertionStore)(nil)

type configMapInterface interface {
	Create(*corev1.ConfigMap) (*corev1.ConfigMap, error)
	Get(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
}

type configMapCacheInterface interface {
	List(namespace string, selector labels.Selector) ([]*corev1.ConfigMap, error)
}

type configMapAssertionStore struct {
	mu             sync.Mutex
	configMaps     configMapInterface
	configMapCache configMapCacheInterface
}

// seen creates a per-assertion ConfigMap on first use and detects replays by its existence.
// Expiry is stored in the ConfigMap so stale entries are cleaned up lazily after process restarts.
func (s *configMapAssertionStore) seen(id string, expiry time.Time) (bool, error) {
	name := assertionConfigMapName(id)
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: samlClientCacheNamespace,
			Labels:    configMapLabels,
		},
		Data: map[string]string{
			"expiry": strconv.FormatInt(expiry.Unix(), 10),
		},
	}
	const maxRetries = 3
	for range maxRetries {
		_, err := s.configMaps.Create(cm)
		if err == nil {
			return false, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return true, err
		}

		// If the ConfigMap already exists, check if it's expired.
		existing, err := s.configMaps.Get(samlClientCacheNamespace, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// Deleted between Create and Get; retry.
			continue
		}
		if err != nil {
			return true, err
		}
		if n, parseErr := strconv.ParseInt(existing.Data["expiry"], 10, 64); parseErr == nil && n <= time.Now().Unix() {
			return false, nil
		}
		return true, nil
	}

	return true, fmt.Errorf("recording SAML assertion: too many retries")
}

func (s *configMapAssertionStore) cleanUpExpiredAssertionIDs(ctx context.Context, c <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			s.mu.Lock()
			configMaps, err := s.configMapCache.List(samlClientCacheNamespace, labels.SelectorFromSet(configMapLabels))
			if err != nil {
				logrus.Errorf("[SAML provider] error listing config maps: %v", err)
				s.mu.Unlock()
				continue
			}

			for _, configMap := range configMaps {
				logrus.Debugf("[SAML provider] checking config map %s for expiry", configMap.Name)
				if expiry, parseErr := strconv.ParseInt(configMap.Data["expiry"], 10, 64); parseErr == nil && expiry <= time.Now().Unix() {
					logrus.Debugf("[SAML provider] deleting expired config map %s", configMap.Name)
					err := s.configMaps.Delete(samlClientCacheNamespace, configMap.Name, &metav1.DeleteOptions{})
					if err != nil {
						logrus.Errorf("[SAML provider] error deleting config map: %v", err)
					}
				}
			}
			s.mu.Unlock()
		}
	}
}

// assertionConfigMapName returns a deterministic, DNS-safe ConfigMap name for an assertion ID.
func assertionConfigMapName(id string) string {
	h := sha256.Sum256([]byte(id))
	return "saml-assertion-" + hex.EncodeToString(h[:16])
}

func newConfigMapIDStore(configMaps configMapInterface, cache configMapCacheInterface) *configMapAssertionStore {
	return &configMapAssertionStore{configMaps: configMaps, configMapCache: cache}
}
