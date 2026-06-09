package params

import (
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPreparePodSpec_WithProxyEnvVars(t *testing.T) {
	// Set up proxy environment variables
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.com:8080")
	t.Setenv("NO_PROXY", "127.0.0.1,localhost,.svc,.cluster.local")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Verify environment variables are set
	assert.Len(t, container.Env, 3, "Should have three environment variables")

	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, "http://proxy.example.com:8080", envMap["HTTP_PROXY"], "HTTP_PROXY should be set")
	assert.Equal(t, "http://proxy.example.com:8080", envMap["HTTPS_PROXY"], "HTTPS_PROXY should be set")
	assert.Equal(t, "127.0.0.1,localhost,.svc,.cluster.local", envMap["NO_PROXY"], "NO_PROXY should be set")
}

func TestPreparePodSpec_WithoutProxyEnvVars(t *testing.T) {
	// Ensure proxy environment variables are not set
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Verify no environment variables are set
	assert.Len(t, container.Env, 0, "Should have no environment variables when proxy vars are not set")
}

func TestPreparePodSpec_WithPartialProxyEnvVars(t *testing.T) {
	// Set only HTTP_PROXY (others remain unset)
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Verify only HTTP_PROXY is set
	assert.Len(t, container.Env, 1, "Should have one environment variable")
	assert.Equal(t, "HTTP_PROXY", container.Env[0].Name, "Should be HTTP_PROXY")
	assert.Equal(t, "http://proxy.example.com:8080", container.Env[0].Value, "HTTP_PROXY value should match")
}

func TestPreparePodSpec_WithEmptyProxyEnvVars(t *testing.T) {
	// Set proxy environment variables to empty strings
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Verify no environment variables are set (empty values should be filtered out)
	assert.Len(t, container.Env, 0, "Should have no environment variables when proxy vars are empty")
}

func TestPreparePodSpec_BasicConfiguration(t *testing.T) {
	// Ensure no proxy vars (t.Setenv with empty string ensures they're unset for this test)
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:v0.4.1",
	}

	podSpec := params.preparePodSpec()

	// Verify basic pod spec configuration
	assert.Equal(t, "rancher-scc-operator-sa", podSpec.ServiceAccountName, "Service account should be rancher-scc-operator-sa")
	assert.Len(t, podSpec.Containers, 1, "Should have one container")

	container := podSpec.Containers[0]
	assert.Equal(t, "scc-operator", container.Name, "Container name should be scc-operator")
	assert.Equal(t, "rancher/scc-operator:v0.4.1", container.Image, "Image should match")
	assert.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy, "Pull policy should be IfNotPresent")

	// Verify security context
	assert.NotNil(t, container.SecurityContext, "Security context should be set")
	assert.NotNil(t, container.SecurityContext.RunAsNonRoot, "RunAsNonRoot should be set")
	assert.True(t, *container.SecurityContext.RunAsNonRoot, "RunAsNonRoot should be true")
	assert.NotNil(t, container.SecurityContext.RunAsUser, "RunAsUser should be set")
	assert.Equal(t, int64(1000), *container.SecurityContext.RunAsUser, "RunAsUser should be 1000")
	assert.NotNil(t, container.SecurityContext.RunAsGroup, "RunAsGroup should be set")
	assert.Equal(t, int64(1000), *container.SecurityContext.RunAsGroup, "RunAsGroup should be 1000")
	assert.NotNil(t, container.SecurityContext.SeccompProfile, "SeccompProfile should be set")
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, container.SecurityContext.SeccompProfile.Type, "SeccompProfile type should be RuntimeDefault")
}

func TestPreparePodSpec_CustomWhitelistedEnvVars(t *testing.T) {
	// Save original whitelist setting
	originalWhitelist := settings.WhitelistEnvironmentVars.Get()
	defer func() {
		// Restore original setting
		settings.WhitelistEnvironmentVars.Set(originalWhitelist)
	}()

	// Set custom whitelist (simulating a user adding custom env vars)
	settings.WhitelistEnvironmentVars.Set("HTTP_PROXY,HTTPS_PROXY,NO_PROXY,CUSTOM_VAR")

	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("CUSTOM_VAR", "custom-value")
	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Verify both whitelisted environment variables are set
	assert.Len(t, container.Env, 2, "Should have two environment variables")

	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, "http://proxy.example.com:8080", envMap["HTTP_PROXY"], "HTTP_PROXY should be set")
	assert.Equal(t, "custom-value", envMap["CUSTOM_VAR"], "CUSTOM_VAR should be set")
}

func TestSetConfigHash_ChangesWithProxyEnvVars(t *testing.T) {
	// Test that hash changes when proxy environment variables change

	// First hash without proxy vars
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	params := &SCCOperatorParams{
		useDeployerOperator: true,
		rancherVersion:      "v2.13.6",
		rancherGitCommit:    "abc123",
		SCCOperatorImage:    "rancher/scc-operator:test",
	}

	err := params.setConfigHash()
	assert.NoError(t, err, "setConfigHash should not error")
	hash1 := params.RefreshHash

	// Second hash with proxy vars
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	err = params.setConfigHash()
	assert.NoError(t, err, "setConfigHash should not error")
	hash2 := params.RefreshHash

	// Hashes should be different
	assert.NotEqual(t, hash1, hash2, "Hash should change when proxy environment variables are added")

	// Third hash with different proxy value
	t.Setenv("HTTP_PROXY", "http://different-proxy.example.com:8080")
	err = params.setConfigHash()
	assert.NoError(t, err, "setConfigHash should not error")
	hash3 := params.RefreshHash

	// Hash should be different from both previous hashes
	assert.NotEqual(t, hash1, hash3, "Hash should be different from original")
	assert.NotEqual(t, hash2, hash3, "Hash should change when proxy value changes")
}

func TestPrepareDeployment_IncludesProxyEnvVars(t *testing.T) {
	// Set up proxy environment variables
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	t.Setenv("HTTPS_PROXY", "https://proxy.example.com:8080")
	t.Setenv("NO_PROXY", "127.0.0.1,localhost,.svc")

	params := &SCCOperatorParams{
		useDeployerOperator: true,
		rancherVersion:      "v2.13.6",
		rancherGitCommit:    "abc123",
		SCCOperatorImage:    "rancher/scc-operator:test",
	}
	err := params.setConfigHash()
	assert.NoError(t, err, "setConfigHash should not error")

	deployment := params.PrepareDeployment()

	// Verify deployment has the proxy environment variables in the pod template
	assert.NotNil(t, deployment, "Deployment should not be nil")
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1, "Should have one container")

	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Len(t, container.Env, 3, "Should have three environment variables")

	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, "http://proxy.example.com:8080", envMap["HTTP_PROXY"], "HTTP_PROXY should be set in deployment")
	assert.Equal(t, "https://proxy.example.com:8080", envMap["HTTPS_PROXY"], "HTTPS_PROXY should be set in deployment")
	assert.Equal(t, "127.0.0.1,localhost,.svc", envMap["NO_PROXY"], "NO_PROXY should be set in deployment")
}

func TestIsValidEnvVarName(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		expected bool
	}{
		// Valid names
		{"valid uppercase", "HTTP_PROXY", true},
		{"valid lowercase", "http_proxy", true},
		{"valid mixed case", "HttpProxy", true},
		{"valid with underscores", "MY_CUSTOM_VAR", true},
		{"valid with numbers", "VAR123", true},
		{"valid starting with underscore", "_PRIVATE_VAR", true},
		{"valid single letter", "X", true},
		{"valid single underscore", "_", true},

		// Invalid names
		{"invalid with hyphen", "HTTP-PROXY", false},
		{"invalid starting with number", "1VAR", false},
		{"invalid with dot", "MY.VAR", false},
		{"invalid with space", "MY VAR", false},
		{"invalid with special char", "VAR!", false},
		{"invalid with at sign", "VAR@HOST", false},
		{"invalid empty", "", false},
		{"invalid with slash", "VAR/NAME", false},
		{"invalid with colon", "VAR:NAME", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEnvVarName(tt.envVar)
			assert.Equal(t, tt.expected, result, "isValidEnvVarName(%q) should return %v", tt.envVar, tt.expected)
		})
	}
}

func TestPreparePodSpec_SkipsInvalidEnvVarNames(t *testing.T) {
	// Save original whitelist setting
	originalWhitelist := settings.WhitelistEnvironmentVars.Get()
	defer func() {
		settings.WhitelistEnvironmentVars.Set(originalWhitelist)
	}()

	// Set whitelist with both valid and invalid variable names
	settings.WhitelistEnvironmentVars.Set("HTTP_PROXY,INVALID-NAME,VALID_VAR,ANOTHER-INVALID")

	// Set environment variables (including invalid names)
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	t.Setenv("INVALID-NAME", "should-be-skipped")
	t.Setenv("VALID_VAR", "valid-value")
	t.Setenv("ANOTHER-INVALID", "also-skipped")

	params := &SCCOperatorParams{
		SCCOperatorImage: "rancher/scc-operator:test",
	}

	podSpec := params.preparePodSpec()

	// Verify container exists
	assert.Len(t, podSpec.Containers, 1, "Should have one container")
	container := podSpec.Containers[0]

	// Should only have 2 valid environment variables (invalid ones skipped)
	assert.Len(t, container.Env, 2, "Should have two environment variables (invalid names skipped)")

	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	// Verify only valid names are included
	assert.Equal(t, "http://proxy.example.com:8080", envMap["HTTP_PROXY"], "HTTP_PROXY should be set")
	assert.Equal(t, "valid-value", envMap["VALID_VAR"], "VALID_VAR should be set")

	// Verify invalid names are NOT included
	_, hasInvalidName := envMap["INVALID-NAME"]
	assert.False(t, hasInvalidName, "INVALID-NAME should be skipped")
	_, hasAnotherInvalid := envMap["ANOTHER-INVALID"]
	assert.False(t, hasAnotherInvalid, "ANOTHER-INVALID should be skipped")
}
