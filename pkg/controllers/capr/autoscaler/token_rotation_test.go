package autoscaler

import (
	"fmt"
	"time"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Test cases for findAutoscalerTokens method

func (s *autoscalerSuite) TestFindAutoscalerTokens_HappyPath_SuccessfulTokenListing() {
	// Create test tokens with autoscaler label
	testTokens := []v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-1",
				Labels: map[string]string{tokens.TokenKindLabel: "autoscaler"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-2",
				Labels: map[string]string{tokens.TokenKindLabel: "autoscaler"},
			},
		},
	}

	tokenList := &v3.TokenList{
		Items: testTokens,
	}

	// Mock token client to return successful response
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Call the method
	tokens, err := s.h.findAutoscalerTokens()

	// Assert the results
	s.NoError(err, "Expected no error when listing tokens successfully")
	s.Len(tokens, 2, "Expected to find 2 autoscaler tokens")
	s.Equal("token-1", tokens[0].Name)
	s.Equal("token-2", tokens[1].Name)
}

func (s *autoscalerSuite) TestFindAutoscalerTokens_Error_FailedToListTokens() {
	// Mock token client to return error
	expectedError := fmt.Errorf("failed to list tokens: connection timeout")
	s.tokenClient.EXPECT().List(gomock.Any()).Return(nil, expectedError)

	// Call the method
	tokens, err := s.h.findAutoscalerTokens()

	// Assert the results
	s.Error(err, "Expected error when token listing fails")
	s.Nil(tokens, "Expected no tokens when listing fails")
	s.Contains(err.Error(), "failed to list autoscaler token", "Error should be wrapped with context")
	s.Contains(err.Error(), "connection timeout", "Original error should be preserved")
}

func (s *autoscalerSuite) TestFindAutoscalerTokens_EdgeCase_EmptyTokenList() {
	// Create empty token list
	tokenList := &v3.TokenList{
		Items: []v3.Token{},
	}

	// Mock token client to return empty list
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Call the method
	tokens, err := s.h.findAutoscalerTokens()

	// Assert the results
	s.NoError(err, "Expected no error when listing empty tokens")
	s.Len(tokens, 0, "Expected to find 0 tokens in empty list")
}

func (s *autoscalerSuite) TestFindAutoscalerTokens_EdgeCase_TokensWithoutAutoscalerLabel() {
	// Create test tokens without autoscaler label
	testTokens := []v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-1",
				Labels: map[string]string{tokens.TokenKindLabel: "user"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-2",
				Labels: map[string]string{tokens.TokenKindLabel: "system"},
			},
		},
	}

	tokenList := &v3.TokenList{
		Items: testTokens,
	}

	// Mock token client to return tokens without autoscaler label
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Call the method
	tokens, err := s.h.findAutoscalerTokens()

	// Assert the results
	s.NoError(err, "Expected no error when listing non-autoscaler tokens")
	s.Len(tokens, 2, "Expected to find all tokens regardless of label (filtering is done by API)")
}

// Test cases for renewToken method

func (s *autoscalerSuite) TestRenewToken_HappyPath_SuccessfulTokenRenewal() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "test-cluster",
			},
		},
		UserID: "user-123",
	}

	// Create test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		// Verify the new token has correct properties
		s.Equal("user-123", token.UserID)
		s.Equal("test-cluster", token.Labels[capi.ClusterNameLabel])
		s.Equal("autoscaler", token.Labels[tokens.TokenKindLabel])
		return token, nil
	})

	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster}, nil)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		secret.Data["value"] = []byte("updated-kubeconfig")
		secret.Data["token"] = []byte("new-token-value")
		return secret, nil
	})
	s.helmOpCache.EXPECT().Get("default", gomock.Any()).Return(nil, errors.NewNotFound(corev1.Resource("helmops"), ""))
	s.helmOp.EXPECT().Create(gomock.Any()).Return(&fleet.HelmOp{}, nil)

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.NoError(err, "Expected no error when renewing token successfully")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToDeleteOldToken() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
		},
	}

	// Mock token client to return error on delete
	expectedError := fmt.Errorf("failed to delete token: not found")
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(expectedError)

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when deleting old token fails")
	s.Contains(err.Error(), "failed to delete old token test-token", "Error should be wrapped with context")
	s.Contains(err.Error(), "not found", "Original error should be preserved")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToGenerateNewToken() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
		},
		UserID: "user-123",
	}

	// Mock token client to succeed on delete but fail on create
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("failed to generate token"))

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when generating new token fails")
	s.Contains(err.Error(), "failed to generate token", "Error should be preserved")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToCreateNewToken() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
		},
		UserID: "user-123",
	}

	// Mock token client to succeed on delete but fail on create
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("failed to create token"))

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when creating new token fails")
	s.Contains(err.Error(), "failed to create renewed token", "Error should be wrapped with context")
	s.Contains(err.Error(), "failed to create token", "Original error should be preserved")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToListClusters() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "test-cluster",
			},
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(&v3.Token{ObjectMeta: testToken.ObjectMeta}, nil)
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return(nil, fmt.Errorf("failed to list clusters"))

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when listing clusters fails")
	s.Contains(err.Error(), "failed to list clusters for token test-token", "Error should be wrapped with context")
	s.Contains(err.Error(), "failed to list clusters", "Original error should be preserved")
}

func (s *autoscalerSuite) TestRenewToken_Error_NoClusterFound() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "nonexistent-cluster",
			},
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(&v3.Token{ObjectMeta: testToken.ObjectMeta}, nil)
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{}, nil)

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when no cluster is found")
	s.Contains(err.Error(), "no cluster found for token test-token with name nonexistent-cluster", "Error should be descriptive")
}

func (s *autoscalerSuite) TestRenewToken_Error_MultipleClustersFound() {
	// Create test token
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "duplicate-cluster",
			},
		},
	}

	// Create multiple clusters with same name
	clusters := []*capi.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "duplicate-cluster",
				Namespace: "ns1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "duplicate-cluster",
				Namespace: "ns2",
			},
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(&v3.Token{ObjectMeta: testToken.ObjectMeta}, nil)
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return(clusters, nil)

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when multiple clusters are found")
	s.Contains(err.Error(), "multiple clusters found for token test-token with name duplicate-cluster", "Error should be descriptive")
	s.Contains(err.Error(), "2 clusters found", "Error should include count")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToUpdateSecret() {
	// Create test token and cluster
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "test-cluster",
			},
		},
	}
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(&v3.Token{}, nil)
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster}, nil)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("failed to update secret"))

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when updating secret fails")
	s.Contains(err.Error(), "failed to update kubeconfig secret for cluster default/test-cluster", "Error should be wrapped with context")
	s.Contains(err.Error(), "failed to update secret", "Original error should be preserved")
}

func (s *autoscalerSuite) TestRenewToken_Error_FailedToCreateHelmOp() {
	// Create test token and cluster
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-token",
			Labels: map[string]string{
				capi.ClusterNameLabel: "test-cluster",
			},
		},
	}
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock expectations
	s.tokenClient.EXPECT().Delete("test-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).Return(&v3.Token{}, nil)
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster}, nil)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).Return(&corev1.Secret{}, nil)
	s.helmOpCache.EXPECT().Get("default", gomock.Any()).Return(nil, errors.NewNotFound(corev1.Resource("helmops"), ""))
	s.helmOp.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("failed to create helm op"))

	// Call the method
	err := s.h.renewToken(testToken)

	// Assert the results
	s.Error(err, "Expected error when creating HelmOp fails")
	s.Contains(err.Error(), "failed to create helm op", "Error should be preserved")
}

// Test cases for updateKubeConfigSecretWithToken method

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_HappyPath_SuccessfulSecretUpdate() {
	// Create test cluster and token
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}
	testToken := "new-token-value"

	// Create existing secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kubeconfig-secret",
			Namespace:       "default",
			ResourceVersion: "v1",
		},
		Data: map[string][]byte{
			"old-value": []byte("old-kubeconfig"),
			"old-token": []byte("old-token"),
		},
	}

	// Mock expectations
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(existingSecret, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		// Verify the secret was updated correctly
		s.Contains(string(secret.Data["value"]), "apiVersion: v1")
		s.Contains(string(secret.Data["value"]), "clusters:")
		s.Contains(string(secret.Data["value"]), "contexts:")
		s.Contains(string(secret.Data["value"]), "users:")
		s.Equal([]byte(testToken), secret.Data["token"], "Token field should be updated")
		s.Equal("v1", secret.ResourceVersion, "Resource version should be preserved")
		return secret, nil
	})

	// Call the method
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, testToken)

	// Assert the results
	s.NoError(err, "Expected no error when updating secret successfully")
	s.NotNil(updatedSecret, "Expected updated secret to be returned")
	s.Equal("kubeconfig-secret", updatedSecret.Name)
	s.Equal("default", updatedSecret.Namespace)
}

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_Error_FailedToGetSecret() {
	// Create test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock secret cache to return error
	expectedError := fmt.Errorf("failed to get secret: not found")
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(nil, expectedError)

	// Call the method
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, "test-token")

	// Assert the results
	s.Error(err, "Expected error when getting secret fails")
	s.Nil(updatedSecret, "Expected no secret when getting fails")
	s.Contains(err.Error(), "failed to get existing kubeconfig secret default/default-test-cluster-autoscaler-kubeconfig", "Error should be wrapped with context")
	s.Contains(err.Error(), "not found", "Original error should be preserved")
}

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_Error_FailedToGenerateKubeconfig() {
	// Create test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Create existing secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}

	// Mock expectations
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(existingSecret, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		// Verify the secret was updated correctly even with empty token
		s.Contains(string(secret.Data["value"]), "apiVersion: v1")
		s.Contains(string(secret.Data["value"]), "clusters:")
		s.Contains(string(secret.Data["value"]), "contexts:")
		s.Contains(string(secret.Data["value"]), "users:")
		s.Equal([]byte(""), secret.Data["token"], "Token field should be empty")
		return secret, nil
	})

	// Call the method with empty token (this doesn't actually fail, but creates a kubeconfig with empty token)
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, "")

	// Assert the results
	s.NoError(err, "Expected no error when generating kubeconfig with empty token")
	s.NotNil(updatedSecret, "Expected updated secret to be returned")
}

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_Error_FailedToUpdateSecret() {
	// Create test cluster and token
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}
	testToken := "new-token-value"

	// Create existing secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}

	// Mock expectations
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(existingSecret, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("failed to update secret"))

	// Call the method
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, testToken)

	// Assert the results
	s.Error(err, "Expected error when updating secret fails")
	s.Nil(updatedSecret, "Expected no secret when update fails")
	s.Contains(err.Error(), "failed to update kubeconfig secret default/default-test-cluster-autoscaler-kubeconfig", "Error should be wrapped with context")
	s.Contains(err.Error(), "failed to update secret", "Original error should be preserved")
}

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_EdgeCase_EmptySecretData() {
	// Create test cluster and token
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}
	testToken := "new-token-value"

	// Create existing secret with empty data
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}

	// Mock expectations
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(existingSecret, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		// Verify the secret was updated correctly even with empty initial data
		// The actual kubeconfig will be a full YAML, not just "new-kubeconfig"
		s.Contains(string(secret.Data["value"]), "apiVersion: v1")
		s.Contains(string(secret.Data["value"]), "clusters:")
		s.Contains(string(secret.Data["value"]), "contexts:")
		s.Contains(string(secret.Data["value"]), "users:")
		s.Equal([]byte(testToken), secret.Data["token"], "Token field should be set")
		return secret, nil
	})

	// Call the method
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, testToken)

	// Assert the results
	s.NoError(err, "Expected no error when updating empty secret")
	s.NotNil(updatedSecret, "Expected updated secret to be returned")
}

func (s *autoscalerSuite) TestUpdateKubeConfigSecretWithToken_EdgeCase_SecretWithMultipleDataFields() {
	// Create test cluster and token
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}
	testToken := "new-token-value"

	// Create existing secret with multiple data fields
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"old-value":  []byte("old-kubeconfig"),
			"old-token":  []byte("old-token"),
			"other-data": []byte("other-value"),
			"more-data":  []byte("more-content"),
		},
	}

	// Mock expectations
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(existingSecret, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		// Verify only the relevant fields were updated, others preserved
		s.Contains(string(secret.Data["value"]), "apiVersion: v1")
		s.Contains(string(secret.Data["value"]), "clusters:")
		s.Contains(string(secret.Data["value"]), "contexts:")
		s.Contains(string(secret.Data["value"]), "users:")
		s.Equal([]byte(testToken), secret.Data["token"], "Token field should be updated")
		s.Equal([]byte("other-value"), secret.Data["other-data"], "Other data should be preserved")
		s.Equal([]byte("more-content"), secret.Data["more-data"], "More data should be preserved")
		return secret, nil
	})

	// Call the method
	updatedSecret, err := s.h.updateKubeConfigSecretWithToken(testCluster, testToken)

	// Assert the results
	s.NoError(err, "Expected no error when updating secret with multiple fields")
	s.NotNil(updatedSecret, "Expected updated secret to be returned")
}

// Test cases for checkAndRenewTokens method

func (s *autoscalerSuite) TestCheckAndRenewTokens_NoTokensFound() {
	// Mock token client to return empty list
	tokenList := &v3.TokenList{
		Items: []v3.Token{},
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.NoError(err, "Expected no error when no tokens are found")
}

func (s *autoscalerSuite) TestCheckAndRenewTokens_TokensFoundNoneExpiring() {
	// Create test tokens that are not expiring (TTLMillis = 0)
	testTokens := []v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-1",
				Labels: map[string]string{tokens.TokenKindLabel: "autoscaler"},
			},
			TTLMillis: 0, // Non-expiring token
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "token-2",
				Labels: map[string]string{tokens.TokenKindLabel: "autoscaler"},
			},
			TTLMillis: 0, // Non-expiring token
		},
	}

	tokenList := &v3.TokenList{
		Items: testTokens,
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.NoError(err, "Expected no error when tokens are found but none are expiring")
}

func (s *autoscalerSuite) TestCheckAndRenewTokens_SingleTokenExpiringSuccessfullyRenewed() {
	// Create a test token that is expiring
	expiredTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339) // Expired 1 day ago
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "expiring-token",
			Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "test-cluster"},
			Annotations: map[string]string{},
		},
		ExpiresAt: expiredTime,
		TTLMillis: 7 * 24 * time.Hour.Milliseconds(), // Expires in 7 days
		UserID:    "user-123",
	}

	tokenList := &v3.TokenList{
		Items: []v3.Token{*testToken},
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Mock cluster for token renewal
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock expectations for token renewal
	s.tokenClient.EXPECT().Delete("expiring-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		s.Equal("user-123", token.UserID)
		s.Equal("test-cluster", token.Labels[capi.ClusterNameLabel])
		s.Equal("autoscaler", token.Labels[tokens.TokenKindLabel])
		return token, nil
	})
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster}, nil)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		secret.Data["value"] = []byte("updated-kubeconfig")
		secret.Data["token"] = []byte("new-token-value")
		return secret, nil
	})
	s.helmOpCache.EXPECT().Get("default", gomock.Any()).Return(nil, errors.NewNotFound(corev1.Resource("helmops"), ""))
	s.helmOp.EXPECT().Create(gomock.Any()).Return(&fleet.HelmOp{}, nil)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.NoError(err, "Expected no error when token is successfully renewed")
}

func (s *autoscalerSuite) TestCheckAndRenewTokens_MultipleTokensOnlyOneExpiring() {
	// Create test tokens - only one is expiring
	expiredTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339) // Expired 1 day ago
	testTokens := []v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "non-expiring-token-1",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-1"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 0, // Non-expiring token
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "expiring-token",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-2"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 7 * 24 * time.Hour.Milliseconds(), // Expires in 7 days
			UserID:    "user-123",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "non-expiring-token-2",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-3"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 0, // Non-expiring token
		},
	}

	tokenList := &v3.TokenList{
		Items: testTokens,
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Mock cluster for the expiring token renewal
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-2",
			Namespace: "default",
		},
	}

	// Mock expectations for token renewal (only for the expiring token)
	s.tokenClient.EXPECT().Delete("expiring-token", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		s.Equal("user-123", token.UserID)
		s.Equal("cluster-2", token.Labels[capi.ClusterNameLabel])
		s.Equal("autoscaler", token.Labels[tokens.TokenKindLabel])
		return token, nil
	})
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster}, nil)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		secret.Data["value"] = []byte("updated-kubeconfig")
		secret.Data["token"] = []byte("new-token-value")
		return secret, nil
	})
	s.helmOpCache.EXPECT().Get("default", gomock.Any()).Return(nil, errors.NewNotFound(corev1.Resource("helmops"), ""))
	s.helmOp.EXPECT().Create(gomock.Any()).Return(&fleet.HelmOp{}, nil)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.NoError(err, "Expected no error when only one token out of multiple is expiring and successfully renewed")
}

func (s *autoscalerSuite) TestCheckAndRenewTokens_MultipleTokensOnlyOneNotExpiring() {
	// Create test tokens - only one is NOT expiring (all others are expiring)
	expiredTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339) // Expired 1 day ago
	testTokens := []v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "expiring-token-1",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-1"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 7 * 24 * time.Hour.Milliseconds(), // Expires in 7 days
			UserID:    "user-123",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "non-expiring-token",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-2"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 0, // Non-expiring token
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "expiring-token-2",
				Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "cluster-3"},
				Annotations: map[string]string{},
			},
			ExpiresAt: expiredTime,
			TTLMillis: 7 * 24 * time.Hour.Milliseconds(), // Expires in 7 days
			UserID:    "user-456",
		},
	}

	tokenList := &v3.TokenList{
		Items: testTokens,
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Mock clusters for token renewal
	testCluster1 := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: "default",
		},
	}
	// Mock expectations for token renewal (for both expiring tokens)
	s.tokenClient.EXPECT().Delete("expiring-token-1", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		s.Equal("user-123", token.UserID)
		s.Equal("cluster-1", token.Labels[capi.ClusterNameLabel])
		return token, nil
	})
	s.tokenClient.EXPECT().Delete("expiring-token-2", gomock.Any()).Return(nil)
	s.tokenClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		s.Equal("user-456", token.UserID)
		s.Equal("cluster-3", token.Labels[capi.ClusterNameLabel])
		return token, nil
	})
	s.capiClusterCache.EXPECT().List("", gomock.Any()).Return([]*capi.Cluster{testCluster1}, nil).Times(2)
	s.secretCache.EXPECT().Get("default", gomock.Any()).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeconfig-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("test-kubeconfig"),
		},
	}, nil).Times(2)
	s.secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		secret.Data["value"] = []byte("updated-kubeconfig")
		secret.Data["token"] = []byte("new-token-value")
		return secret, nil
	}).Times(2)
	s.helmOpCache.EXPECT().Get("default", gomock.Any()).Return(nil, errors.NewNotFound(corev1.Resource("helmops"), "")).Times(2)
	s.helmOp.EXPECT().Create(gomock.Any()).Return(&fleet.HelmOp{}, nil).Times(2)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.NoError(err, "Expected no error when multiple tokens are expiring and successfully renewed")
}

func (s *autoscalerSuite) TestCheckAndRenewTokens_TokenRenewalFails() {
	// Create a test token that is expiring
	expiredTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339) // Expired 1 day ago
	testToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "expiring-token",
			Labels:      map[string]string{tokens.TokenKindLabel: "autoscaler", capi.ClusterNameLabel: "test-cluster"},
			Annotations: map[string]string{},
		},
		ExpiresAt: expiredTime,
		TTLMillis: 7 * 24 * time.Hour.Milliseconds(), // Expires in 7 days
		UserID:    "user-123",
	}

	tokenList := &v3.TokenList{
		Items: []v3.Token{*testToken},
	}
	s.tokenClient.EXPECT().List(gomock.Any()).Return(tokenList, nil)

	// Mock expectations for token renewal that fails
	expectedError := fmt.Errorf("failed to delete old token")
	s.tokenClient.EXPECT().Delete("expiring-token", gomock.Any()).Return(expectedError)

	// Call the method
	err := s.h.checkAndRenewTokens()

	// Assert the results
	s.Error(err, "Expected error when token renewal fails")
	s.Contains(err.Error(), "failed to delete old token expiring-token: failed to delete old token", "Error should mention failed token deletion")
}
