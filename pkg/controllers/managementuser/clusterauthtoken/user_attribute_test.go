package clusterauthtoken

import (
	"testing"

	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	clusterFakes "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3/fakes"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserAttributeHandlerSyncCopiesOnlyUserExtraKeys(t *testing.T) {
	t.Parallel()

	userExtraOnly := map[string]map[string][]string{
		"okta": {
			"principalid": {"okta_user://alice"},
			"username":    {"alice@example.com"},
		},
	}
	userAttribute := &managementv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "u-abcde"},
		ExtraByProvider: map[string]map[string][]string{
			"okta": {
				"principalid": {"okta_user://alice"},
				"username":    {"alice@example.com"},
				"externalid":  {"00u123"},
				"email":       {"alice@example.com"},
			},
		},
	}

	tests := []struct {
		name       string
		stored     map[string]map[string][]string
		wantUpdate bool
	}{
		{
			name:       "stored copy has other keys",
			stored:     userAttribute.ExtraByProvider,
			wantUpdate: true,
		},
		{
			name:       "stored copy has no extras",
			stored:     nil,
			wantUpdate: true,
		},
		{
			name:       "stored copy differs only in other keys",
			stored:     userExtraOnly,
			wantUpdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clusterUserAttribute := &clusterv3.ClusterUserAttribute{
				ObjectMeta:      metav1.ObjectMeta{Name: "u-abcde", Namespace: "cattle-system"},
				ExtraByProvider: tt.stored,
			}
			var updated *clusterv3.ClusterUserAttribute
			handler := &userAttributeHandler{
				namespace: "cattle-system",
				clusterUserAttributeLister: &clusterFakes.ClusterUserAttributeListerMock{
					GetFunc: func(namespace string, name string) (*clusterv3.ClusterUserAttribute, error) {
						return clusterUserAttribute, nil
					},
				},
				clusterUserAttribute: &clusterFakes.ClusterUserAttributeInterfaceMock{
					UpdateFunc: func(in *clusterv3.ClusterUserAttribute) (*clusterv3.ClusterUserAttribute, error) {
						updated = in
						return in, nil
					},
				},
			}

			_, err := handler.Sync("u-abcde", userAttribute)
			require.NoError(t, err)

			if !tt.wantUpdate {
				assert.Nil(t, updated)
				return
			}
			require.NotNil(t, updated)
			assert.Equal(t, userExtraOnly, updated.ExtraByProvider)
		})
	}
}

func TestUserExtraByProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   map[string]map[string][]string
		want map[string]map[string][]string
	}{
		{
			name: "nil",
			in:   nil,
			want: nil,
		},
		{
			name: "only other keys",
			in: map[string]map[string][]string{
				"okta": {"externalid": {"00u123"}, "email": {"alice@example.com"}},
			},
			want: nil,
		},
		{
			name: "several providers",
			in: map[string]map[string][]string{
				"okta":   {"principalid": {"okta_user://alice"}, "email": {"alice@example.com"}},
				"github": {"username": {"alice-gh"}},
				"azure":  {"externalid": {"123"}},
			},
			want: map[string]map[string][]string{
				"okta":   {"principalid": {"okta_user://alice"}},
				"github": {"username": {"alice-gh"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, userExtraByProvider(tt.in))
		})
	}
}
