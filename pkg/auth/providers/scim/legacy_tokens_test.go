package scim

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestTokenSecretNamespace(t *testing.T) {
	assert.Equal(t, namespace.System, tokenSecretNamespace)
}

func TestDeleteLegacyTokens(t *testing.T) {
	ctrl := gomock.NewController(t)

	wantSelector := labels.Set{secretKindLabel: scimAuthToken}.AsSelector().String()

	captureLog := func(t *testing.T) *bytes.Buffer {
		var buf bytes.Buffer
		out, level := logrus.StandardLogger().Out, logrus.GetLevel()
		logrus.SetOutput(&buf)
		logrus.SetLevel(logrus.InfoLevel)
		t.Cleanup(func() {
			logrus.SetOutput(out)
			logrus.SetLevel(level)
		})
		return &buf
	}

	legacySecrets := func(names ...string) *v1.SecretList {
		list := &v1.SecretList{}
		for _, name := range names {
			list.Items = append(list.Items, v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace.GlobalNamespace}})
		}
		return list
	}

	t.Run("legacy tokens are deleted and logged", func(t *testing.T) {
		buf := captureLog(t)

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(ns string, opts metav1.ListOptions) (*v1.SecretList, error) {
			assert.Equal(t, namespace.GlobalNamespace, ns)
			assert.Equal(t, wantSelector, opts.LabelSelector)
			return legacySecrets("scim-okta", "scim-azuread"), nil
		}).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-okta", gomock.Any()).Return(nil).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-azuread", gomock.Any()).Return(nil).Times(1)

		DeleteLegacyTokens(secrets)

		assert.Contains(t, buf.String(), "scim-okta")
		assert.Contains(t, buf.String(), "scim-azuread")
		assert.NotContains(t, buf.String(), "level=error")
	})

	t.Run("already deleted by another replica", func(t *testing.T) {
		buf := captureLog(t)

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(legacySecrets("scim-okta"), nil).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-okta", gomock.Any()).
			Return(apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "scim-okta")).Times(1)

		DeleteLegacyTokens(secrets)

		assert.Empty(t, buf.String())
	})

	t.Run("delete is conditional on the listed secret's UID", func(t *testing.T) {
		captureLog(t)

		list := legacySecrets("scim-okta")
		list.Items[0].UID = "uid-1"

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(list, nil).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-okta", gomock.Any()).
			DoAndReturn(func(_, _ string, opts *metav1.DeleteOptions) error {
				require.NotNil(t, opts.Preconditions)
				require.NotNil(t, opts.Preconditions.UID)
				assert.Equal(t, types.UID("uid-1"), *opts.Preconditions.UID)
				return nil
			}).Times(1)

		DeleteLegacyTokens(secrets)
	})

	t.Run("a failed delete doesn't stop the others", func(t *testing.T) {
		buf := captureLog(t)

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(legacySecrets("scim-okta", "scim-azuread"), nil).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-okta", gomock.Any()).Return(fmt.Errorf("some error")).Times(1)
		secrets.EXPECT().Delete(namespace.GlobalNamespace, "scim-azuread", gomock.Any()).Return(nil).Times(1)

		DeleteLegacyTokens(secrets)

		assert.Contains(t, buf.String(), "level=error")
		assert.Contains(t, buf.String(), "scim-azuread")
	})

	t.Run("no legacy tokens", func(t *testing.T) {
		buf := captureLog(t)

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.SecretList{}, nil).Times(1)

		DeleteLegacyTokens(secrets)

		assert.Empty(t, buf.String())
	})

	t.Run("list fails", func(t *testing.T) {
		buf := captureLog(t)

		secrets := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
		secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error")).Times(1)

		DeleteLegacyTokens(secrets)

		assert.Contains(t, buf.String(), "level=error")
	})
}
