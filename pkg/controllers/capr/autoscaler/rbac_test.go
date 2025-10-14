package autoscaler

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func (s *autoscalerSuite) TestEnsureUser_UserDoesNotExist() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock user cache to return not found error
	s.userCache.EXPECT().Get(autoscalerUserName(testCluster)).Return(nil, errors.NewNotFound(v3.Resource("user"), "user"))

	// Mock user creation
	expectedUser := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: autoscalerUserName(testCluster),
		},
		Username: autoscalerUserName(testCluster),
	}
	s.user.EXPECT().Create(gomock.AssignableToTypeOf(&v3.User{})).Return(expectedUser, nil)

	// Call ensureUser and verify it creates the user
	user, err := s.h.ensureUser(testCluster)
	s.Require().NoError(err, "Should not return error when creating user")
	s.Require().NotNil(user, "Should return a user object")
	s.Require().Equal(autoscalerUserName(testCluster), user.Username, "Username should match expected")
}

func (s *autoscalerSuite) TestEnsureUser_UserAlreadyExists() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Mock user cache to return existing user
	existingUser := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: autoscalerUserName(testCluster),
		},
		Username: autoscalerUserName(testCluster),
	}
	s.userCache.EXPECT().Get(autoscalerUserName(testCluster)).Return(existingUser, nil)

	// Call ensureUser and verify it returns the existing user without creating
	user, err := s.h.ensureUser(testCluster)
	s.Require().NoError(err, "Should not return error when user exists")
	s.Require().NotNil(user, "Should return a user object")
	s.Require().Equal(existingUser, user, "Should return the existing user object")
}

func (s *autoscalerSuite) TestEnsureGlobalRole_RoleDoesNotExist() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Create test machine deployments and machines
	testMachineDeployments := []*capi.MachineDeployment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "md-1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "md-2",
				Namespace: "default",
			},
		},
	}

	testMachines := []*capi.Machine{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-2",
				Namespace: "default",
			},
		},
	}

	// Mock global role cache to return not found error
	s.globalRoleCache.EXPECT().Get(globalRoleName(testCluster)).Return(nil, errors.NewNotFound(v3.Resource("globalrole"), "globalrole"))

	// Mock global role creation
	expectedRole := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleName(testCluster),
			OwnerReferences: ownerReference(testCluster),
		},
		DisplayName: fmt.Sprintf("Autoscaler Global Role [%v]", testCluster.Name),
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			"default": {
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machinedeployments"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: []string{"md-1", "md-2"},
				},
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machinedeployments/scale"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: []string{"md-1", "md-2"},
				},
				{
					APIGroups:     []string{"cluster.x-k8s.io"},
					Resources:     []string{"machines"},
					Verbs:         []string{"get", "update", "patch"},
					ResourceNames: []string{"machine-1", "machine-2"},
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.x-k8s.io"},
				Resources: []string{
					"machinedeployments",
					"machinepools",
					"machines",
					"machinesets",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
	s.globalRole.EXPECT().Create(gomock.AssignableToTypeOf(&v3.GlobalRole{})).Return(expectedRole, nil)

	// Call ensureGlobalRole and verify it creates the role
	role, err := s.h.ensureGlobalRole(testCluster, testMachineDeployments, testMachines)
	s.Require().NoError(err, "Should not return error when creating global role")
	s.Require().NotNil(role, "Should return a global role object")
	s.Require().Equal(globalRoleName(testCluster), role.Name, "Role name should match expected")
}

func (s *autoscalerSuite) TestEnsureGlobalRole_RoleAlreadyExists() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	// Create test machine deployments and machines
	testMachineDeployments := []*capi.MachineDeployment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "md-1",
				Namespace: "default",
			},
		},
	}

	testMachines := []*capi.Machine{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-1",
				Namespace: "default",
			},
		},
	}

	// Mock existing global role
	existingRole := &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleName(testCluster),
			OwnerReferences: ownerReference(testCluster),
		},
		DisplayName:     "Existing Role",
		NamespacedRules: map[string][]rbacv1.PolicyRule{},
		Rules:           []rbacv1.PolicyRule{},
	}
	s.globalRoleCache.EXPECT().Get(globalRoleName(testCluster)).Return(existingRole, nil)

	// Mock global role update
	expectedUpdatedRole := existingRole.DeepCopy()
	expectedUpdatedRole.NamespacedRules = map[string][]rbacv1.PolicyRule{
		"default": {
			{
				APIGroups:     []string{"cluster.x-k8s.io"},
				Resources:     []string{"machinedeployments"},
				Verbs:         []string{"get", "update", "patch"},
				ResourceNames: []string{"md-1"},
			},
			{
				APIGroups:     []string{"cluster.x-k8s.io"},
				Resources:     []string{"machinedeployments/scale"},
				Verbs:         []string{"get", "update", "patch"},
				ResourceNames: []string{"md-1"},
			},
			{
				APIGroups:     []string{"cluster.x-k8s.io"},
				Resources:     []string{"machines"},
				Verbs:         []string{"get", "update", "patch"},
				ResourceNames: []string{"machine-1"},
			},
		},
	}
	expectedUpdatedRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"cluster.x-k8s.io"},
			Resources: []string{
				"machinedeployments",
				"machinepools",
				"machines",
				"machinesets",
			},
			Verbs: []string{"get", "list", "watch"},
		},
	}
	s.globalRole.EXPECT().Update(expectedUpdatedRole).Return(expectedUpdatedRole, nil)

	// Call ensureGlobalRole and verify it updates the existing role
	role, err := s.h.ensureGlobalRole(testCluster, testMachineDeployments, testMachines)
	s.Require().NoError(err, "Should not return error when updating existing global role")
	s.Require().NotNil(role, "Should return a global role object")
	s.Require().Equal(existingRole.Name, role.Name, "Should return the existing role name")
	s.Require().Equal("Existing Role", role.DisplayName, "Display name should remain unchanged")
}

func (s *autoscalerSuite) TestEnsureGlobalRoleBinding_BindingDoesNotExist() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"
	globalRoleName := "test-global-role"

	// Mock global role binding cache to return not found error
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName(testCluster)).Return(nil, errors.NewNotFound(v3.Resource("globalrolebinding"), "globalrolebinding"))

	// Mock global role binding creation
	expectedBinding := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleBindingName(testCluster),
			OwnerReferences: ownerReference(testCluster),
		},
		GlobalRoleName: globalRoleName,
		UserName:       username,
	}
	s.globalRoleBinding.EXPECT().Create(gomock.AssignableToTypeOf(&v3.GlobalRoleBinding{})).Return(expectedBinding, nil)

	// Call ensureGlobalRoleBinding and verify it creates the binding
	err := s.h.ensureGlobalRoleBinding(testCluster, username, globalRoleName)
	s.Require().NoError(err, "Should not return error when creating global role binding")
}

func (s *autoscalerSuite) TestEnsureGlobalRoleBinding_BindingAlreadyExistsWithSameValues() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"
	globalRoleName := "test-global-role"

	// Mock existing global role binding with same values
	existingBinding := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleBindingName(testCluster),
			OwnerReferences: ownerReference(testCluster),
		},
		GlobalRoleName: globalRoleName,
		UserName:       username,
	}
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName(testCluster)).Return(existingBinding, nil)

	// Call ensureGlobalRoleBinding and verify it returns without updating
	err := s.h.ensureGlobalRoleBinding(testCluster, username, globalRoleName)
	s.Require().NoError(err, "Should not return error when binding exists with same values")
}

func (s *autoscalerSuite) TestEnsureGlobalRoleBinding_BindingAlreadyExistsWithDifferentValues() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"
	globalRoleName := "test-global-role"

	// Mock existing global role binding with different values
	existingBinding := &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            globalRoleBindingName(testCluster),
			OwnerReferences: ownerReference(testCluster),
		},
		GlobalRoleName: "different-role",
		UserName:       "different-user",
	}
	s.globalRoleBindingCache.EXPECT().Get(globalRoleBindingName(testCluster)).Return(existingBinding, nil)

	// Mock global role binding update
	expectedUpdatedBinding := existingBinding.DeepCopy()
	expectedUpdatedBinding.UserName = username
	expectedUpdatedBinding.GlobalRoleName = globalRoleName
	s.globalRoleBinding.EXPECT().Update(expectedUpdatedBinding).Return(expectedUpdatedBinding, nil)

	// Call ensureGlobalRoleBinding and verify it updates the binding
	err := s.h.ensureGlobalRoleBinding(testCluster, username, globalRoleName)
	s.Require().NoError(err, "Should not return error when updating existing global role binding")
}

func (s *autoscalerSuite) TestEnsureUserToken_TokenDoesNotExist() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"

	// Mock token cache to return not found error
	s.tokenCache.EXPECT().Get(username).Return(nil, errors.NewNotFound(v3.Resource("token"), "token"))

	// Mock token creation - we can't predict the random token value, so just check that Create is called
	s.token.EXPECT().Create(gomock.AssignableToTypeOf(&v3.Token{})).DoAndReturn(func(token *v3.Token) (*v3.Token, error) {
		// Verify the generated token has the correct structure
		s.Require().Equal(username, token.Name, "Token name should match username")
		s.Require().Equal(username, token.UserID, "Token UserID should match username")
		s.Require().Equal("local", token.AuthProvider, "Token AuthProvider should be local")
		s.Require().True(token.IsDerived, "Token should be derived")
		s.Require().NotEmpty(token.Token, "Token should not be empty")
		return token, nil
	}).Return(&v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:            username,
			OwnerReferences: ownerReference(testCluster),
		},
		UserID:       username,
		AuthProvider: "local",
		IsDerived:    true,
		Token:        "generated-token-string", // This will be the actual generated token
	}, nil)

	// Call ensureUserToken and verify it creates the token
	result, err := s.h.ensureUserToken(testCluster, username)
	s.Require().NoError(err, "Should not return error when creating token")
	s.Require().NotNil(result, "Should return a result")
	s.Require().Contains(result, username+":", "Result should contain username followed by colon")
}

func (s *autoscalerSuite) TestEnsureUserToken_TokenAlreadyExists() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"

	// Mock existing token
	existingToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:            username,
			OwnerReferences: ownerReference(testCluster),
		},
		Token: "existing-token-string",
	}
	s.tokenCache.EXPECT().Get(username).Return(existingToken, nil)

	// Call ensureUserToken and verify it returns the existing token without creating
	result, err := s.h.ensureUserToken(testCluster, username)
	s.Require().NoError(err, "Should not return error when token exists")
	s.Require().NotNil(result, "Should return a result")
	s.Require().Equal("test-user:existing-token-string", result, "Result should match expected format with existing token")
}

func (s *autoscalerSuite) TestEnsureUserToken_CacheError() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	username := "test-user"

	// Mock token cache to return an error (not not found)
	cacheError := fmt.Errorf("cache connection failed")
	s.tokenCache.EXPECT().Get(username).Return(nil, cacheError)

	// Call ensureUserToken and verify it returns the error
	result, err := s.h.ensureUserToken(testCluster, username)
	s.Require().Error(err, "Should return error when cache fails")
	s.Require().Empty(result, "Should not return result when cache fails")
	s.Require().Equal(cacheError, err, "Error should match the cache error")
}

func (s *autoscalerSuite) TestCreateKubeConfigSecretUsingTemplate_SecretDoesNotExist() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	testToken := "test-token-string"

	// Mock secret cache to return not found error
	s.secretCache.EXPECT().Get(testCluster.Namespace, kubeconfigSecretName(testCluster)).Return(nil, errors.NewNotFound(corev1.Resource("secrets"), "secret"))

	// Mock generateKubeconfig function call by calling the actual function
	actualKubeconfigData, err := generateKubeconfig(testToken)
	s.Require().NoError(err, "Should not error generating kubeconfig")

	s.secret.EXPECT().Create(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
		// Verify the secret has the correct structure
		s.Require().Equal(testCluster.Namespace, secret.Namespace, "Secret namespace should match cluster namespace")
		s.Require().Equal(kubeconfigSecretName(testCluster), secret.Name, "Secret name should match expected kubeconfig secret name")
		s.Require().Equal(ownerReference(testCluster), secret.OwnerReferences, "Owner references should match")

		// Verify annotations
		expectedAnnotations := map[string]string{
			"provisioning.cattle.io/sync":                  "true",
			"provisioning.cattle.io/sync-target-namespace": "kube-system",
			"provisioning.cattle.io/sync-target-name":      "mgmt-kubeconfig",
			"rke.cattle.io/object-authorized-for-clusters": testCluster.Name,
		}
		s.Require().Equal(expectedAnnotations, secret.Annotations, "Annotations should match expected")

		// Verify labels
		expectedLabels := map[string]string{
			capi.ClusterNameLabel:                    testCluster.Name,
			"provisioning.cattle.io/kubeconfig-type": "autoscaler",
		}
		s.Require().Equal(expectedLabels, secret.Labels, "Labels should match expected")

		// Verify data
		s.Require().Contains(secret.Data, "value", "Secret should contain value key")
		s.Require().Contains(secret.Data, "token", "Secret should contain token key")
		s.Require().Equal(actualKubeconfigData, secret.Data["value"], "Value should match generated kubeconfig data")
		s.Require().Equal([]byte(testToken), secret.Data["token"], "Token should match expected token")

		return secret, nil
	}).Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testCluster.Namespace,
			Name:            kubeconfigSecretName(testCluster),
			OwnerReferences: ownerReference(testCluster),
			Annotations: map[string]string{
				"provisioning.cattle.io/sync":                  "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "mgmt-kubeconfig",
				"rke.cattle.io/object-authorized-for-clusters": testCluster.Name,
			},
			Labels: map[string]string{
				capi.ClusterNameLabel:                    testCluster.Name,
				"provisioning.cattle.io/kubeconfig-type": "autoscaler",
			},
		},
		Data: map[string][]byte{
			"value": actualKubeconfigData,
			"token": []byte(testToken),
		},
	}, nil)

	// Call createKubeConfigSecretUsingTemplate and verify it creates the secret
	secret, err := s.h.ensureKubeconfigSecretUsingTemplate(testCluster, testToken)
	s.Require().NoError(err, "Should not return error when creating kubeconfig secret")
	s.Require().NotNil(secret, "Should return a secret object")
	s.Require().Equal(kubeconfigSecretName(testCluster), secret.Name, "Secret name should match expected")
}

func (s *autoscalerSuite) TestCreateKubeConfigSecretUsingTemplate_SecretAlreadyExists() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	testToken := "test-token-string"

	// Mock existing secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testCluster.Namespace,
			Name:            kubeconfigSecretName(testCluster),
			OwnerReferences: ownerReference(testCluster),
			Annotations: map[string]string{
				"provisioning.cattle.io/sync":                  "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "mgmt-kubeconfig",
				"rke.cattle.io/object-authorized-for-clusters": testCluster.Name,
			},
			Labels: map[string]string{
				capi.ClusterNameLabel:                    testCluster.Name,
				"provisioning.cattle.io/kubeconfig-type": "autoscaler",
			},
		},
		Data: map[string][]byte{
			"value": []byte("existing-kubeconfig-data"),
			"token": []byte(testToken),
		},
	}
	s.secretCache.EXPECT().Get(testCluster.Namespace, kubeconfigSecretName(testCluster)).Return(existingSecret, nil)

	// Call createKubeConfigSecretUsingTemplate and verify it returns the existing secret without creating
	secret, err := s.h.ensureKubeconfigSecretUsingTemplate(testCluster, testToken)
	s.Require().NoError(err, "Should not return error when secret exists")
	s.Require().NotNil(secret, "Should return a secret object")
	s.Require().Equal(existingSecret, secret, "Should return the existing secret object")
}

func (s *autoscalerSuite) TestCreateKubeConfigSecretUsingTemplate_CacheError() {
	// Create a test cluster
	testCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	testToken := "test-token-string"

	// Mock secret cache to return an error (not not found)
	cacheError := fmt.Errorf("cache connection failed")
	s.secretCache.EXPECT().Get(testCluster.Namespace, kubeconfigSecretName(testCluster)).Return(nil, cacheError)

	// Call createKubeConfigSecretUsingTemplate and verify it returns the error
	secret, err := s.h.ensureKubeconfigSecretUsingTemplate(testCluster, testToken)
	s.Require().Error(err, "Should return error when cache fails")
	s.Require().Nil(secret, "Should not return secret when cache fails")
	s.Require().Equal(cacheError, err, "Error should match the cache error")
}
