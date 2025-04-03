package oidcprovider

import (
	"encoding/json"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestOnChange(t *testing.T) {
	ctlr := gomock.NewController(t)
	const (
		fakeOIDCClientName = "client-name"
		fakeClientId       = "client-id"
		fakeClientSecret   = "client-secret"
		fakeOIDCClientUID  = "uid"
	)
	type mockParams struct {
		secretCache     *fake.MockCacheInterface[*v1.Secret]
		secretClient    *fake.MockClientInterface[*v1.Secret, *v1.SecretList]
		oidcClientCache *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient]
		oidcClient      *fake.MockNonNamespacedClientInterface[*v3.OIDCClient, *v3.OIDCClientList]
		generator       *mocks.MockClientIDAndSecretGenerator
	}

	tests := map[string]struct {
		oidcClient  *v3.OIDCClient
		setupMock   func(*mockParams)
		expectedErr string
	}{
		"clientID and clientSecret are created for a new OIDCClient": {
			oidcClient: &v3.OIDCClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeOIDCClientName,
					UID:  fakeOIDCClientUID,
				},
			},
			setupMock: func(p *mockParams) {
				p.generator.EXPECT().GenerateClientID().Return(fakeClientId, nil)
				patchData := map[string]interface{}{
					"status": map[string]string{
						"clientID": fakeClientId,
					},
				}
				patchBytes, _ := json.Marshal(patchData)
				p.oidcClientCache.EXPECT().List(labels.Everything()).Return(nil, nil)
				p.oidcClient.EXPECT().Patch(fakeOIDCClientName, types.MergePatchType, patchBytes).Return(nil, nil)
				p.secretCache.EXPECT().Get(secretNamespace, fakeClientId).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				p.generator.EXPECT().GenerateClientSecret().Return(fakeClientSecret, nil)
				p.secretClient.EXPECT().Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					StringData: map[string]string{
						secretKeyPrefix + "1": fakeClientSecret,
					},
				}).Return(nil, nil)
			},
		},

		"clientID and clientSecret are not created for an existing OIDCClient": {
			oidcClient: &v3.OIDCClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeOIDCClientName,
					UID:  fakeOIDCClientUID,
				},
				Status: v3.OIDCClientStatus{
					ClientID: fakeClientId,
				},
			},
			setupMock: func(p *mockParams) {
				p.secretCache.EXPECT().Get(secretNamespace, fakeClientId).Return(&v1.Secret{}, nil)
			},
		},
		"new client secret is created with annotation": {
			oidcClient: &v3.OIDCClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeOIDCClientName,
					UID:  fakeOIDCClientUID,
					Annotations: map[string]string{
						createClientSecretAnn: "true",
					},
				},
				Status: v3.OIDCClientStatus{
					ClientID: fakeClientId,
				},
			},

			setupMock: func(p *mockParams) {
				p.generator.EXPECT().GenerateClientSecret().Return(fakeClientSecret, nil)
				p.secretCache.EXPECT().Get(secretNamespace, fakeClientId).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "1": []byte(fakeClientSecret),
					},
				}, nil)
				p.secretClient.EXPECT().Update(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "1": []byte(fakeClientSecret),
						secretKeyPrefix + "2": []byte(fakeClientSecret),
					},
				}).Return(nil, nil)
				p.oidcClient.EXPECT().Update(&v3.OIDCClient{
					ObjectMeta: metav1.ObjectMeta{
						Name:        fakeOIDCClientName,
						UID:         fakeOIDCClientUID,
						Annotations: map[string]string{},
					},
					Status: v3.OIDCClientStatus{
						ClientID: fakeClientId,
					},
				}).Return(nil, nil)
			},
		},
		"client secret is regenerated with annotation": {
			oidcClient: &v3.OIDCClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeOIDCClientName,
					UID:  fakeOIDCClientUID,
					Annotations: map[string]string{
						regenerateClientSecretAnn: secretKeyPrefix + "1",
					},
				},
				Status: v3.OIDCClientStatus{
					ClientID: fakeClientId,
				},
			},
			setupMock: func(p *mockParams) {
				p.generator.EXPECT().GenerateClientSecret().Return(fakeClientSecret, nil)
				p.secretCache.EXPECT().Get(secretNamespace, fakeClientId).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "1": []byte("oldSecret"),
					},
				}, nil)
				p.secretClient.EXPECT().Update(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "1": []byte(fakeClientSecret),
					},
				}).Return(nil, nil)
				p.oidcClient.EXPECT().Update(&v3.OIDCClient{
					ObjectMeta: metav1.ObjectMeta{
						Name:        fakeOIDCClientName,
						UID:         fakeOIDCClientUID,
						Annotations: map[string]string{},
					},
					Status: v3.OIDCClientStatus{
						ClientID: fakeClientId,
					},
				}).Return(nil, nil)
			},
		},
		"client secret is removed with annotation": {
			oidcClient: &v3.OIDCClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeOIDCClientName,
					UID:  fakeOIDCClientUID,
					Annotations: map[string]string{
						removeClientSecretAnn: secretKeyPrefix + "1",
					},
				},
				Status: v3.OIDCClientStatus{
					ClientID: fakeClientId,
				},
			},
			setupMock: func(p *mockParams) {
				p.secretCache.EXPECT().Get(secretNamespace, fakeClientId).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "1": []byte(fakeClientSecret),
						secretKeyPrefix + "2": []byte(fakeClientSecret),
					},
				}, nil)
				p.secretClient.EXPECT().Update(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeClientId,
						Namespace: secretNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "management.cattle.io/v3",
								Kind:       "OIDCClient",
								Name:       fakeOIDCClientName,
								UID:        fakeOIDCClientUID,
							},
						},
					},
					Data: map[string][]byte{
						secretKeyPrefix + "2": []byte(fakeClientSecret),
					},
				}).Return(nil, nil)
				p.oidcClient.EXPECT().Update(&v3.OIDCClient{
					ObjectMeta: metav1.ObjectMeta{
						Name:        fakeOIDCClientName,
						UID:         fakeOIDCClientUID,
						Annotations: map[string]string{},
					},
					Status: v3.OIDCClientStatus{
						ClientID: fakeClientId,
					},
				}).Return(nil, nil)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mocks := &mockParams{
				secretCache:     fake.NewMockCacheInterface[*v1.Secret](ctlr),
				secretClient:    fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr),
				oidcClientCache: fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctlr),
				oidcClient:      fake.NewMockNonNamespacedClientInterface[*v3.OIDCClient, *v3.OIDCClientList](ctlr),
				generator:       mocks.NewMockClientIDAndSecretGenerator(ctlr),
			}
			if test.setupMock != nil {
				test.setupMock(mocks)
			}

			c := oidcClientController{
				secretClient:    mocks.secretClient,
				secretCache:     mocks.secretCache,
				oidcClient:      mocks.oidcClient,
				oidcClientCache: mocks.oidcClientCache,
				generator:       mocks.generator,
			}

			_, err := c.onChange("", test.oidcClient)

			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
