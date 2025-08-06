package pbkdf2

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha3"
	"encoding/json"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	LocalUserPasswordsNamespace = "cattle-local-user-passwords"
	iterations                  = 210000
	keyLength                   = 32
	passwordHashAnnotation      = "cattle.io/password-hash"
	pbkdf2sha3512Hash           = "pbkdf2sha3512"
	bcryptHash                  = "bcrypt"
)

// Pbkdf2 handles password storage and hashing using PBKDF2.
type Pbkdf2 struct {
	secretLister  v1.SecretCache
	secretClient  v1.SecretClient
	hashKey       func(password string, salt []byte, iter, keyLength int) ([]byte, error)
	saltGenerator func() ([]byte, error)
}

func New(secretLister v1.SecretCache, secretClient v1.SecretClient) *Pbkdf2 {
	return &Pbkdf2{
		secretLister:  secretLister,
		secretClient:  secretClient,
		hashKey:       sha3512Key,
		saltGenerator: generateSalt,
	}
}

// CreatePassword hashes the provided password using PBKDF2 and stores it in a secret associated with the specified user.
func (p *Pbkdf2) CreatePassword(user *v3.User, password string) error {
	salt, err := p.saltGenerator()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}
	hashedPassword, err := p.hashKey(password, salt, iterations, keyLength)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = p.secretClient.Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: LocalUserPasswordsNamespace,
			Annotations: map[string]string{
				passwordHashAnnotation: pbkdf2sha3512Hash,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       user.Name,
					UID:        user.UID,
					APIVersion: "management.cattle.io/v3",
					Kind:       "User",
				},
			},
		},
		Data: map[string][]byte{
			"password": hashedPassword,
			"salt":     salt,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// UpdatePassword hashes the provided password using PBKDF2 and updates the secret associated with the specified user
func (p *Pbkdf2) UpdatePassword(userId string, newPassword string) error {
	secret, err := p.secretLister.Get(LocalUserPasswordsNamespace, userId)
	if err != nil {
		return fmt.Errorf("failed to get password secret: %w", err)
	}
	salt, err := p.saltGenerator()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	hashedNewPassword, err := p.hashKey(newPassword, salt, iterations, keyLength)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:   "replace",
		Path: "/data",
		Value: map[string][]byte{
			"password": hashedNewPassword,
			"salt":     salt,
		},
	}})
	if err != nil {
		return err
	}
	_, err = p.secretClient.Patch(LocalUserPasswordsNamespace, secret.Name, types.JSONPatchType, patch)
	if err != nil {
		return fmt.Errorf("failed to patch secret: %w", err)
	}

	return nil
}

// VerifyAndUpdatePassword hashes the provided password using PBKDF2 and updates the secret associated with the specified user
// if the currentPassword matches the password stored.
func (p *Pbkdf2) VerifyAndUpdatePassword(userId string, currentPassword, newPassword string) error {
	secret, err := p.secretLister.Get(LocalUserPasswordsNamespace, userId)
	if err != nil {
		return fmt.Errorf("failed to get password secret: %w", err)
	}

	hashedPassword, err := p.hashKey(currentPassword, secret.Data["salt"], iterations, keyLength)
	if !bytes.Equal(hashedPassword, secret.Data["password"]) {
		return fmt.Errorf("invalid current password")
	}

	return p.UpdatePassword(userId, newPassword)
}

// VerifyPassword verifies if the password stored is the same as the password provided.
// if the password stored is using the legacy hashing algorithm (bcrypt) it will be updated to PBKDF2.
func (p *Pbkdf2) VerifyPassword(user *v3.User, password string) error {
	secret, err := p.secretLister.Get(LocalUserPasswordsNamespace, user.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get password secret: %w", err)
	}
	if apierrors.IsNotFound(err) {
		if user.Password == "" {
			return fmt.Errorf("failed to get password")
		}
		// This will only be reached if migration failed. This code will be removed in 2.14
		logrus.Warn("Using old password field. Check if User migration failed!")
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			return err
		}
		return nil
	}

	switch secret.Annotations[passwordHashAnnotation] {
	case pbkdf2sha3512Hash:
		hashedPassword, err := p.hashKey(password, secret.Data["salt"], iterations, keyLength)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		if !bytes.Equal(hashedPassword, secret.Data["password"]) {
			return fmt.Errorf("invalid password")
		}
		return nil
	case bcryptHash:
		if err := bcrypt.CompareHashAndPassword(secret.Data["password"], []byte(password)); err != nil {
			return err
		}
		// migrate password to pkbf2 hashing algorithm
		salt, err := p.saltGenerator()
		if err != nil {
			return fmt.Errorf("failed to generate salt: %w", err)
		}
		hashedNewPassword, err := p.hashKey(password, salt, iterations, keyLength)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{
			{
				Op:   "replace",
				Path: "/data",
				Value: map[string][]byte{
					"password": hashedNewPassword,
					"salt":     salt,
				},
			},
			{
				Op:    "replace",
				Path:  "/metadata/annotations/" + passwordHashAnnotation,
				Value: pbkdf2sha3512Hash,
			},
		})
		if err != nil {
			return err
		}
		_, err = p.secretClient.Patch(LocalUserPasswordsNamespace, secret.Name, types.JSONPatchType, patch)
		if err != nil {
			return fmt.Errorf("failed to patch secret: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("unsupported hashing algorithm")
	}
}

func sha3512Key(password string, salt []byte, iter, keyLength int) ([]byte, error) {
	return pbkdf2.Key(sha3.New512, password, salt, iter, keyLength)
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	return salt, nil
}
