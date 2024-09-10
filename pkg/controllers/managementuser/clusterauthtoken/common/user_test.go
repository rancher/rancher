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
	clusterAuthToken, err := NewClusterAuthToken(&token, hashedValue)
	assert.Nil(t, err)
	assert.Nil(t, VerifyClusterAuthToken(token.Token, clusterAuthToken))
}

func TestInvalidPassword(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	clusterAuthToken, _ := NewClusterAuthToken(&token, hashedValue)
	assert.NotNil(t, VerifyClusterAuthToken(token.Token+":wrong!", clusterAuthToken))
}

func TestExpired(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = time.Now().Add(-time.Minute).Format(time.RFC3339)
	clusterAuthToken, _ := NewClusterAuthToken(&token, hashedValue)
	assert.NotNil(t, VerifyClusterAuthToken(token.Token, clusterAuthToken))
}

func TestNotExpired(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = time.Now().Add(time.Minute).Format(time.RFC3339)
	clusterAuthToken, _ := NewClusterAuthToken(&token, hashedValue)
	assert.Nil(t, VerifyClusterAuthToken(token.Token, clusterAuthToken))
}

func TestInvalidExpiresAt(t *testing.T) {
	token := getToken()
	hasher := hashers.GetHasher()
	hashedValue, err := hasher.CreateHash(token.Token)
	assert.NoError(t, err, "got an error but did not expect one")
	token.ExpiresAt = "some invalid time stamp"
	clusterAuthToken, _ := NewClusterAuthToken(&token, hashedValue)
	assert.NotNil(t, VerifyClusterAuthToken(token.Token, clusterAuthToken))
}
