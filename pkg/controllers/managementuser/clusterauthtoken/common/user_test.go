package common

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/stretchr/testify/assert"

	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

func getToken() managementv3.Token {
	longPassword := strings.Repeat("A", 72)
	token := managementv3.Token{
		Token:  longPassword,
		UserID: "me",
	}
	return token
}

func TestValidUser(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthTokenSecret := NewClusterAuthTokenSecret(&token, hashedValue)
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, clusterAuthTokenSecret)
	assert.Nil(t, err)
	assert.False(t, migrate)
}

func TestUnmigrated(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthToken.SecretKeyHash = hashedValue
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, nil)
	assert.Nil(t, err)
	assert.True(t, migrate)
}

func TestMissingSecret(t *testing.T) {
	token := getToken()
	clusterAuthToken := NewClusterAuthToken(&token)
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, nil)
	assert.NotNil(t, err)
	assert.False(t, migrate)
}

func TestInvalidPassword(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthTokenSecret := NewClusterAuthTokenSecret(&token, hashedValue)
	err, migrate := VerifyClusterAuthToken(token.Token+":wrong!", clusterAuthToken, clusterAuthTokenSecret)
	assert.NotNil(t, err)
	assert.False(t, migrate)
}

func TestExpired(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = time.Now().Add(-time.Minute).Format(time.RFC3339)
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthTokenSecret := NewClusterAuthTokenSecret(&token, hashedValue)
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, clusterAuthTokenSecret)
	assert.NotNil(t, err)
	assert.False(t, migrate)
}

func TestNotExpired(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = time.Now().Add(time.Minute).Format(time.RFC3339)
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthTokenSecret := NewClusterAuthTokenSecret(&token, hashedValue)
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, clusterAuthTokenSecret)
	assert.Nil(t, err)
	assert.False(t, migrate)
}

func TestInvalidExpiresAt(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = "some invalid time stamp"
	clusterAuthToken := NewClusterAuthToken(&token)
	clusterAuthTokenSecret := NewClusterAuthTokenSecret(&token, hashedValue)
	err, migrate := VerifyClusterAuthToken(token.Token, clusterAuthToken, clusterAuthTokenSecret)
	assert.NotNil(t, err)
	assert.False(t, migrate)
}
