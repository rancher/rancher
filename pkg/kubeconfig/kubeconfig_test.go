package kubeconfig

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const testHost = "rancher.example.com"

func loadConfig(t *testing.T, content string) *clientcmdapi.Config {
	t.Helper()

	config, err := clientcmd.Load([]byte(content))
	require.NoError(t, err)

	return config
}

func TestForTokenBased(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		token    string
		execUser *ExecUser
		wantArgs []string
	}{
		{
			name:     "token command without exec user",
			wantArgs: []string{"token", "--server=" + testHost, "--user=downstream"},
		},
		{
			name:     "get-token command with exec user",
			execUser: &ExecUser{ID: "u-w7drc", AuthProvider: "github"},
			wantArgs: []string{"auth", "get-token", "--server=" + testHost, "--user-id=u-w7drc", "--auth-provider=github"},
		},
		{
			name:     "get-token command without auth provider",
			execUser: &ExecUser{ID: "u-w7drc"},
			wantArgs: []string{"auth", "get-token", "--server=" + testHost, "--user-id=u-w7drc"},
		},
		{
			name:     "embedded token ignores exec user",
			token:    "kubeconfig-u-w7drc:secret",
			execUser: &ExecUser{ID: "u-w7drc", AuthProvider: "github"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := ForTokenBased("downstream", "c-abc12", testHost, tt.token, tt.execUser)
			require.NoError(t, err)

			config := loadConfig(t, content)
			require.Len(t, config.AuthInfos, 1)
			authInfo := config.AuthInfos["downstream"]
			require.NotNil(t, authInfo)
			assert.Equal(t, "downstream", config.Contexts["downstream"].AuthInfo)

			if tt.token != "" {
				assert.Equal(t, tt.token, authInfo.Token)
				assert.Nil(t, authInfo.Exec)
				return
			}

			assert.Empty(t, authInfo.Token)
			require.NotNil(t, authInfo.Exec)
			assert.Equal(t, "rancher", authInfo.Exec.Command)
			assert.Equal(t, "client.authentication.k8s.io/v1beta1", authInfo.Exec.APIVersion)
			assert.Equal(t, tt.wantArgs, authInfo.Exec.Args)
		})
	}
}

func TestForClusterTokenBased(t *testing.T) {
	t.Parallel()

	nodes := []*normanv3.Node{
		{
			Spec: apiv3.NodeSpec{ControlPlane: true, RequestedHostname: "ace-cp1"},
			Status: apiv3.NodeStatus{InternalNodeStatus: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: "InternalIP", Address: "10.0.0.1"}},
			}},
		},
		{Spec: apiv3.NodeSpec{RequestedHostname: "ace-worker1"}},
	}

	tests := []struct {
		name         string
		cluster      *clientv3.Cluster
		token        string
		execUser     *ExecUser
		wantContexts []string
		wantArgs     []string
	}{
		{
			name:         "token command with control plane nodes",
			cluster:      &clientv3.Cluster{Name: "ace", CACert: "Y2E=", LocalClusterAuthEndpoint: &clientv3.LocalClusterAuthEndpoint{Enabled: true}},
			wantContexts: []string{"ace", "ace-cp1"},
			wantArgs:     []string{"token", "--server=" + testHost, "--user=ace", "--cluster=c-m-ace"},
		},
		{
			name:         "get-token command with control plane nodes",
			cluster:      &clientv3.Cluster{Name: "ace", CACert: "Y2E=", LocalClusterAuthEndpoint: &clientv3.LocalClusterAuthEndpoint{Enabled: true}},
			execUser:     &ExecUser{ID: "u-w7drc", AuthProvider: "local"},
			wantContexts: []string{"ace", "ace-cp1"},
			wantArgs:     []string{"auth", "get-token", "--server=" + testHost, "--cluster=c-m-ace", "--user-id=u-w7drc", "--auth-provider=local"},
		},
		{
			name: "get-token command with FQDN",
			cluster: &clientv3.Cluster{Name: "ace", LocalClusterAuthEndpoint: &clientv3.LocalClusterAuthEndpoint{
				Enabled: true,
				FQDN:    "ace.example.com",
				CACerts: "cacert",
			}},
			execUser:     &ExecUser{ID: "u-w7drc", AuthProvider: "local"},
			wantContexts: []string{"ace", "ace-fqdn"},
			wantArgs:     []string{"auth", "get-token", "--server=" + testHost, "--cluster=c-m-ace", "--user-id=u-w7drc", "--auth-provider=local"},
		},
		{
			name:         "get-token command without auth provider",
			cluster:      &clientv3.Cluster{Name: "ace", CACert: "Y2E=", LocalClusterAuthEndpoint: &clientv3.LocalClusterAuthEndpoint{Enabled: true}},
			execUser:     &ExecUser{ID: "u-w7drc"},
			wantContexts: []string{"ace", "ace-cp1"},
			wantArgs:     []string{"auth", "get-token", "--server=" + testHost, "--cluster=c-m-ace", "--user-id=u-w7drc"},
		},
		{
			name:         "embedded token ignores exec user",
			cluster:      &clientv3.Cluster{Name: "ace", CACert: "Y2E=", LocalClusterAuthEndpoint: &clientv3.LocalClusterAuthEndpoint{Enabled: true}},
			token:        "kubeconfig-u-w7drc:secret",
			execUser:     &ExecUser{ID: "u-w7drc", AuthProvider: "local"},
			wantContexts: []string{"ace", "ace-cp1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := ForClusterTokenBased(tt.cluster, nodes, "c-m-ace", testHost, tt.token, tt.execUser)
			require.NoError(t, err)

			config := loadConfig(t, content)
			require.Len(t, config.AuthInfos, 1)
			authInfo := config.AuthInfos["ace"]
			require.NotNil(t, authInfo)

			require.Len(t, config.Contexts, len(tt.wantContexts))
			for _, name := range tt.wantContexts {
				require.Contains(t, config.Contexts, name)
				assert.Equal(t, "ace", config.Contexts[name].AuthInfo)
			}

			if tt.token != "" {
				assert.Equal(t, tt.token, authInfo.Token)
				assert.Nil(t, authInfo.Exec)
				return
			}

			assert.Empty(t, authInfo.Token)
			require.NotNil(t, authInfo.Exec)
			assert.Equal(t, "rancher", authInfo.Exec.Command)
			assert.Equal(t, tt.wantArgs, authInfo.Exec.Args)
		})
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()

	newConfig := func(token string, execUser *ExecUser) KubeConfig {
		return KubeConfig{
			Meta: Meta{Name: "kubeconfig-abcde"},
			Clusters: []Cluster{
				{Name: "rancher", Server: "https://" + testHost},
				{Name: "downstream", Server: "https://" + testHost + "/k8s/clusters/c-abc12"},
				{Name: "ace", Server: "https://" + testHost + "/k8s/clusters/c-m-ace"},
			},
			Users: []User{
				{Name: "rancher", Token: token, Host: testHost},
				{Name: "ace", Token: token, Host: testHost, ClusterID: "c-m-ace"},
			},
			Contexts: []Context{
				{Name: "rancher", Cluster: "rancher", User: "rancher"},
				{Name: "downstream", Cluster: "downstream", User: "rancher"},
				{Name: "ace", Cluster: "ace", User: "ace"},
			},
			CurrentContext: "rancher",
			ExecUser:       execUser,
		}
	}

	tests := []struct {
		name     string
		token    string
		execUser *ExecUser
		wantArgs map[string][]string
	}{
		{
			name: "token command without exec user",
			wantArgs: map[string][]string{
				"rancher": {"token", "--server=" + testHost, "--user=rancher"},
				"ace":     {"token", "--server=" + testHost, "--user=ace", "--cluster=c-m-ace"},
			},
		},
		{
			name:     "get-token command with exec user",
			execUser: &ExecUser{ID: "u-w7drc", AuthProvider: "activedirectory"},
			wantArgs: map[string][]string{
				"rancher": {"auth", "get-token", "--server=" + testHost, "--user-id=u-w7drc", "--auth-provider=activedirectory"},
				"ace":     {"auth", "get-token", "--server=" + testHost, "--cluster=c-m-ace", "--user-id=u-w7drc", "--auth-provider=activedirectory"},
			},
		},
		{
			name:     "get-token command without auth provider",
			execUser: &ExecUser{ID: "u-w7drc"},
			wantArgs: map[string][]string{
				"rancher": {"auth", "get-token", "--server=" + testHost, "--user-id=u-w7drc"},
				"ace":     {"auth", "get-token", "--server=" + testHost, "--cluster=c-m-ace", "--user-id=u-w7drc"},
			},
		},
		{
			name:     "embedded token ignores exec user",
			token:    "kubeconfig-u-w7drc:secret",
			execUser: &ExecUser{ID: "u-w7drc", AuthProvider: "activedirectory"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := Generate(newConfig(tt.token, tt.execUser))
			require.NoError(t, err)

			config := loadConfig(t, content)
			require.Len(t, config.AuthInfos, 2)
			require.Len(t, config.Contexts, 3)
			assert.Equal(t, "rancher", config.Contexts["rancher"].AuthInfo)
			assert.Equal(t, "rancher", config.Contexts["downstream"].AuthInfo)
			assert.Equal(t, "ace", config.Contexts["ace"].AuthInfo)

			for _, name := range []string{"rancher", "ace"} {
				authInfo := config.AuthInfos[name]
				require.NotNil(t, authInfo, name)

				if tt.token != "" {
					assert.Equal(t, tt.token, authInfo.Token, name)
					assert.Nil(t, authInfo.Exec, name)
					continue
				}

				assert.Empty(t, authInfo.Token, name)
				require.NotNil(t, authInfo.Exec, name)
				assert.Equal(t, "rancher", authInfo.Exec.Command, name)
				assert.Equal(t, tt.wantArgs[name], authInfo.Exec.Args, name)
			}
		})
	}
}
