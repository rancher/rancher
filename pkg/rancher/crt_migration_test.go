package rancher

import (
	"errors"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1wrangler "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type stubCoreV1ForCRTMigration struct {
	corev1wrangler.Interface
	configMapCtrl corev1wrangler.ConfigMapController
	secretCtrl    corev1wrangler.SecretController
}

func (s *stubCoreV1ForCRTMigration) ConfigMap() corev1wrangler.ConfigMapController {
	return s.configMapCtrl
}

func (s *stubCoreV1ForCRTMigration) Secret() corev1wrangler.SecretController {
	return s.secretCtrl
}

type stubMgmtV3ForCRTMigration struct {
	managementv3.Interface
	crtCtrl managementv3.ClusterRegistrationTokenController
}

func (s *stubMgmtV3ForCRTMigration) ClusterRegistrationToken() managementv3.ClusterRegistrationTokenController {
	return s.crtCtrl
}

// TestMigrateCRTTokensToSecretsOrphanedNamespace covers CRTs left behind in a
// cluster namespace that no longer exists. Creating the token secret fails
// with NotFound; the migration must skip those CRTs and complete, rather than
// return the error and stop Rancher from starting.
func TestMigrateCRTTokensToSecretsOrphanedNamespace(t *testing.T) {
	originalVersion := rancherversion.Version
	rancherversion.Version = "v2.15.3" // migrations are skipped for "dev"
	t.Cleanup(func() { rancherversion.Version = originalVersion })

	const orphanedNamespace = "c-bddqq"

	setup := func(t *testing.T, createErr error) (*wrangler.Context, *ctrlfake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList]) {
		ctrl := gomock.NewController(t)

		configMaps := ctrlfake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		cmCache := ctrlfake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
		configMaps.EXPECT().Cache().Return(cmCache)
		cmCache.EXPECT().Get(cattleNamespace, migrateCRTTokensToSecrets).
			Return(nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, migrateCRTTokensToSecrets))

		crts := ctrlfake.NewMockControllerInterface[*mgmtv3.ClusterRegistrationToken, *mgmtv3.ClusterRegistrationTokenList](ctrl)
		crts.EXPECT().List("", metav1.ListOptions{}).Return(&mgmtv3.ClusterRegistrationTokenList{
			Items: []mgmtv3.ClusterRegistrationToken{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-token",
						Namespace: orphanedNamespace,
						UID:       "1234",
					},
					Spec:   mgmtv3.ClusterRegistrationTokenSpec{ClusterName: orphanedNamespace},
					Status: mgmtv3.ClusterRegistrationTokenStatus{Token: "plaintext-token"},
				},
			},
		}, nil)
		// The orphaned CRT must not be updated; gomock fails on any unexpected call.

		secrets := ctrlfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			assert.Equal(t, orphanedNamespace, s.Namespace)
			assert.Equal(t, "crt-token-default-token", s.Name)
			return nil, createErr
		})

		return &wrangler.Context{
			Core: &stubCoreV1ForCRTMigration{configMapCtrl: configMaps, secretCtrl: secrets},
			Mgmt: &stubMgmtV3ForCRTMigration{crtCtrl: crts},
		}, configMaps
	}

	t.Run("skips CRTs whose namespace is missing and records completion", func(t *testing.T) {
		// This is the error the kube-apiserver NamespaceLifecycle admission
		// plugin returns for a Create into a namespace that does not exist.
		w, configMaps := setup(t, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, orphanedNamespace))
		configMaps.EXPECT().Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migrateCRTTokensToSecrets,
				Namespace: cattleNamespace,
			},
			Data: map[string]string{crtTokensToSecretsMigratedKey: "true"},
		})

		require.NoError(t, migrateCRTTokensToSecretsFunc(w))
	})

	t.Run("other create errors abort without recording completion", func(t *testing.T) {
		w, _ := setup(t, apierrors.NewInternalError(errors.New("etcd unavailable")))
		// No Create/Update is expected on configMaps.

		err := migrateCRTTokensToSecretsFunc(w)
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})
}
