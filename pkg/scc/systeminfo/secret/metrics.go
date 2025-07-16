package secret

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/controllers/repos"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
)

var metricSecretMu sync.Mutex

type MetricsSecretManager struct {
	secretNamespace string
	secretRepo      *repos.SecretRepo
}

func New(
	namespace string,
	secretRepo *repos.SecretRepo,

) *MetricsSecretManager {
	return &MetricsSecretManager{
		secretNamespace: namespace,
		secretRepo:      secretRepo,
	}
}

func (m *MetricsSecretManager) UpdateMetricsDebugSecret(byteData []byte) error {
	metricSecretMu.Lock()
	defer metricSecretMu.Unlock()

	desiredSecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.SCCMetricsOutputSecretName,
			Namespace: m.secretNamespace,
			Annotations: map[string]string{
				"secret.kubernetes.io/managed-by":        "scc-operator",
				"secret.kubernetes.io/overwrite-warning": "Managed by scc-operator. Manual edits will be overwritten.",
			},
			Labels: map[string]string{
				consts.LabelSccManagedBy: "scc-operator",
			},
		},
		Data: map[string][]byte{
			"scc-metrics.json": byteData,
		},
	}

	_, createOrUpdateErr := m.secretRepo.CreateOrUpdateSecret(&desiredSecret)
	if createOrUpdateErr != nil {
		return createOrUpdateErr
	}

	return nil
}

func (m *MetricsSecretManager) Remove() error {
	metricSecretMu.Lock()
	defer metricSecretMu.Unlock()
	currentSecret, err := m.secretRepo.SecretsCache.Get(m.secretNamespace, consts.SCCMetricsOutputSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	delErr := m.secretRepo.Secrets.Delete(currentSecret.Namespace, currentSecret.Name, &metav1.DeleteOptions{})
	if delErr != nil && apierrors.IsNotFound(delErr) {
		return nil
	}
	return delErr
}
