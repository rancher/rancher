package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAdd(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		fakeCode = "fake-code"
	)

	tests := map[string]struct {
		data           map[string]Session
		inputSession   Session
		inputCode      string
		secretCache    func() corev1.SecretCache
		secretClient   func(s Session) corev1.SecretClient
		expectedErrMsg string
	}{
		"code is not present": {
			data: map[string]Session{},
			inputSession: Session{
				ClientID: "client-id",
			},
			inputCode: fakeCode,
			secretCache: func() corev1.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctrl)
				mock.EXPECT().Get(namespace, fakeCode).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

				return mock
			},
			secretClient: func(s Session) corev1.SecretClient {
				sessionBytes, _ := json.Marshal(s)
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeCode,
						Namespace: namespace,
						Labels: map[string]string{
							secretLabel: "true",
						},
					},
					Data: map[string][]byte{
						secretKey: sessionBytes,
					},
				}).Return(&v1.Secret{}, nil)

				return mock
			},
		},
		"code is already present": {
			data: map[string]Session{},
			inputSession: Session{
				ClientID: "client-id",
			},
			inputCode: fakeCode,
			secretCache: func() corev1.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctrl)
				mock.EXPECT().Get(namespace, fakeCode).Return(&v1.Secret{}, nil)

				return mock
			},
			secretClient: func(s Session) corev1.SecretClient {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
			},
			expectedErrMsg: "code already exists",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &SecretSessionStore{
				secretCache:  test.secretCache(),
				secretClient: test.secretClient(test.inputSession),
				expiryTime:   time.Hour,
			}

			err := store.Add(test.inputCode, test.inputSession)

			if test.expectedErrMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedErrMsg)
			}
		})
	}
}

func TestGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	now := time.Now()
	fakeSession := Session{
		ClientID:  "client-id",
		TokenName: "token-name",
		Nonce:     "nonce",
		CreatedAt: now,
	}
	fakeCode := "code123"
	tests := map[string]struct {
		secretClient    func() corev1.SecretClient
		inputCode       string
		expectedSession Session
		expectedErrMsg  string
	}{
		"code is present": {
			inputCode:       fakeCode,
			expectedSession: fakeSession,
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				sessionBytes, _ := json.Marshal(fakeSession)
				mock.EXPECT().Get(namespace, fakeCode, metav1.GetOptions{}).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeCode,
						Namespace: namespace,
						Labels: map[string]string{
							secretLabel: "true",
						},
					},
					Data: map[string][]byte{
						secretKey: sessionBytes,
					},
				}, nil)

				return mock
			},
		},
		"code is not present": {
			inputCode: fakeCode,
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Get(namespace, fakeCode, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "")).Times(4) // we retry four times

				return mock
			},
			expectedErrMsg: "invalid code",
		},
		"code expired": {
			inputCode: fakeCode,
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				sessionBytes, _ := json.Marshal(Session{CreatedAt: time.Unix(0, 0)})
				mock.EXPECT().Get(namespace, fakeCode, metav1.GetOptions{}).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeCode,
						Namespace: namespace,
						Labels: map[string]string{
							secretLabel: "true",
						},
					},
					Data: map[string][]byte{
						secretKey: sessionBytes,
					},
				}, nil)

				return mock
			},
			expectedErrMsg: "the code has expired",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &SecretSessionStore{
				secretClient: test.secretClient(),
				expiryTime:   time.Hour,
				mu:           sync.Mutex{},
			}

			session, err := store.Get(test.inputCode)

			if test.expectedErrMsg == "" {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedSession.Nonce, session.Nonce)
				assert.Equal(t, test.expectedSession.ClientID, session.ClientID)
				assert.Equal(t, test.expectedSession.TokenName, session.TokenName)
				assert.True(t, test.expectedSession.CreatedAt.Equal(session.CreatedAt))
			} else {
				assert.ErrorContains(t, err, test.expectedErrMsg)
			}
		})
	}
}

func TestCleanUpExpiredSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessionExpiredBytes, _ := json.Marshal(&Session{})
	sessionNonExpired := &Session{
		CreatedAt: time.Now(),
	}
	sessionNonExpiredBytes, _ := json.Marshal(sessionNonExpired)
	sessionExpiredSecret := &v1.Secret{
		Data: map[string][]byte{
			secretKey: sessionExpiredBytes,
		},
	}
	sessionNonExpiredSecret := &v1.Secret{
		Data: map[string][]byte{
			secretKey: sessionNonExpiredBytes,
		},
	}

	tests := map[string]struct {
		secretCache  func() corev1.SecretCache
		secretClient func() corev1.SecretClient
	}{
		"remove expired session": {
			secretCache: func() corev1.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctrl)
				mock.EXPECT().List(namespace, labels.Set{secretLabel: "true"}.AsSelector()).Return([]*v1.Secret{
					sessionExpiredSecret,
				}, nil)

				return mock
			},
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Delete(namespace, sessionExpiredSecret.Name, &metav1.DeleteOptions{}).Return(nil)

				return mock
			},
		},
		"remove only expired session": {
			secretCache: func() corev1.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctrl)
				mock.EXPECT().List(namespace, labels.Set{secretLabel: "true"}.AsSelector()).Return([]*v1.Secret{
					sessionNonExpiredSecret,
					sessionExpiredSecret,
				}, nil)

				return mock
			},
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Delete(namespace, sessionExpiredSecret.Name, &metav1.DeleteOptions{}).Return(nil)

				return mock
			},
		},
		"don't remove if there aren't any expired sessions": {
			secretCache: func() corev1.SecretCache {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctrl)
				mock.EXPECT().List(namespace, labels.Set{secretLabel: "true"}.AsSelector()).Return([]*v1.Secret{
					sessionNonExpiredSecret,
				}, nil)

				return mock
			},
			secretClient: func() corev1.SecretClient {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			storage := &SecretSessionStore{
				secretClient: test.secretClient(),
				secretCache:  test.secretCache(),
				expiryTime:   time.Hour,
			}
			ctx, cancel := context.WithCancel(context.TODO())
			c := make(chan time.Time)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				storage.cleanUpExpiredSessions(ctx, c)
			}()

			c <- time.Unix(0, 0)
			cancel()
			wg.Wait()
		})
	}
}

func TestRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	fakeCode := "code123"
	tests := map[string]struct {
		secretClient   func() corev1.SecretClient
		inputCode      string
		expectedErrMsg string
	}{
		"success delete": {
			inputCode: fakeCode,
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Delete(namespace, fakeCode, &metav1.DeleteOptions{}).Return(nil)

				return mock
			},
		},
		"delete failure": {
			inputCode: fakeCode,
			secretClient: func() corev1.SecretClient {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctrl)
				mock.EXPECT().Delete(namespace, fakeCode, &metav1.DeleteOptions{}).Return(fmt.Errorf("unexpected error"))

				return mock
			},
			expectedErrMsg: "unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &SecretSessionStore{
				secretClient: test.secretClient(),
				expiryTime:   time.Hour,
				mu:           sync.Mutex{},
			}

			err := store.Remove(test.inputCode)

			if test.expectedErrMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErrMsg)
			}
		})
	}
}
