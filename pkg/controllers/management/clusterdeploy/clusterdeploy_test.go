package clusterdeploy

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRegistrationTokenHashChanged(t *testing.T) {
	cluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	tests := []struct {
		name         string
		crtStatus    *apimgmtv3.ClusterRegistrationTokenStatus
		wantChanged  bool
		wantNotReady bool
	}{
		{name: "no system CRT", crtStatus: nil},
		{name: "system CRT token not in secret yet", crtStatus: &apimgmtv3.ClusterRegistrationTokenStatus{Token: "legacy"}, wantNotReady: true},
		{name: "system CRT token in secret", crtStatus: &apimgmtv3.ClusterRegistrationTokenStatus{TokenSecretName: "crt-token-system"}, wantChanged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &clusterDeploy{
				crtLister: &fakes.ClusterRegistrationTokenListerMock{
					GetFunc: func(namespace, name string) (*apimgmtv3.ClusterRegistrationToken, error) {
						if tt.crtStatus == nil {
							return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusterregistrationtokens"}, name)
						}
						return &apimgmtv3.ClusterRegistrationToken{
							ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
							Status:     *tt.crtStatus,
						}, nil
					},
				},
				secretLister: &corefakes.SecretListerMock{
					GetFunc: func(namespace, name string) (*corev1.Secret, error) {
						return &corev1.Secret{Data: map[string][]byte{"token": []byte("abc")}}, nil
					},
				},
			}

			changed, err := cd.registrationTokenHashChanged(cluster)
			if tt.wantNotReady {
				require.ErrorIs(t, err, crt.ErrTokenNotReady)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantChanged, changed)
		})
	}
}
