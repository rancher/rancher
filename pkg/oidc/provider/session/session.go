package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	namespace   = "cattle-oidc-codes"
	secretKey   = "session"
	secretLabel = "cattle.io/oidc-code"
)

// Session holds information provided in the authorize endpoint that will be used in the token endpoint.
type Session struct {
	// ClientID represents the OIDC client id
	ClientID string
	// TokenName is the Rancher token name
	TokenName string
	// Scope is the OIDC scope
	Scope []string
	// CodeChallenge is the PKCE code challenge
	CodeChallenge string
	//Nonce is the OIDC nonce
	Nonce string
	//CreatedAt represents when the session was created
	CreatedAt time.Time
}

// SecretSessionStore stores auth sessions in k8s secrets. The name of the secret is the code generated in the authorize endpoint,
// and the session contains the information provided in the authorize endpoint.
type SecretSessionStore struct {
	secretCache  corecontrollers.SecretCache
	secretClient corecontrollers.SecretClient
	expiryTime   time.Duration
	mu           sync.Mutex
}

// NewSecretSessionStore creates a new SecretSessionStore
func NewSecretSessionStore(ctx context.Context, secretCache corecontrollers.SecretCache, secretClient corecontrollers.SecretClient, expiryTime time.Duration) *SecretSessionStore {
	storage := &SecretSessionStore{
		secretCache:  secretCache,
		secretClient: secretClient,
		expiryTime:   expiryTime,
	}
	t := time.NewTicker(expiryTime)
	// codes are valid for a maximum of 10 minutes. Therefore, we need to clean the expired sessions associated with these codes.
	go storage.cleanUpExpiredSessions(ctx, t.C)

	return storage
}

// Add stores a session referenced by a code in a k8s secret.
func (m *SecretSessionStore) Add(code string, session Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.secretCache.Get(namespace, code)
	if err == nil {
		return fmt.Errorf("code already exists")
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("error getting code: %v", err)
	}
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("error marshalling session: %v", err)
	}
	_, err = m.secretClient.Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      code,
			Namespace: namespace,
			Labels: map[string]string{
				secretLabel: "true",
			},
		},
		Data: map[string][]byte{
			secretKey: sessionBytes,
		},
	})
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}

	return nil
}

// GetAndRemove retrieves the session associated with the given code.
func (m *SecretSessionStore) Get(code string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var secret *corev1.Secret
	// Retry if the secret is not available yet. In most cases (if not all), the secret will be available, even if it was created on a different node.
	err := wait.ExponentialBackoff(retry.DefaultBackoff, func() (bool, error) {
		var err error
		secret, err = m.secretClient.Get(namespace, code, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid code: %v", err)
	}

	var session Session
	err = json.Unmarshal(secret.Data[secretKey], &session)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling session: %v", err)
	}

	if time.Since(session.CreatedAt) > m.expiryTime {
		return nil, fmt.Errorf("the code has expired")
	}

	return &session, nil
}

// Remove removes the session associated with the given code.
func (m *SecretSessionStore) Remove(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.secretClient.Delete(namespace, code, &metav1.DeleteOptions{})
}

func (m *SecretSessionStore) cleanUpExpiredSessions(ctx context.Context, c <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			m.mu.Lock()
			secrets, err := m.secretCache.List(namespace, labels.Set{secretLabel: "true"}.AsSelector())
			if err != nil {
				logrus.Errorf("[OIDC provider] error listing secrets: %v", err)
				return
			}
			for _, secret := range secrets {
				var session Session
				err = json.Unmarshal(secret.Data[secretKey], &session)
				if err != nil {
					logrus.Errorf("[OIDC provider] error unmarshalling session: %v", err)
				}
				if time.Since(session.CreatedAt) > m.expiryTime {
					err := m.secretClient.Delete(namespace, secret.Name, &metav1.DeleteOptions{})
					if err != nil {
						logrus.Errorf("[OIDC provider] error deleting secret: %v", err)
					}
				}
			}
			m.mu.Unlock()
		}
	}
}
