package clusterdeploy

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestClusterDeploy_redeployAgent(t *testing.T) {
	testClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	},
	}
	testUpdatedClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	}, {
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/etcd-test",
		Value:  "true",
	},
	}

	tests := []struct {
		name             string
		cluster          *v3.Cluster
		expectedRedeploy bool
	}{
		{
			name: "test-default",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-default",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
				Status: v3.ClusterStatus{
					AppliedAgentEnvVars: settings.DefaultAgentSettingsAsEnvVars(),
				},
			},
			expectedRedeploy: false,
		},
		{
			name: "test-add-cluster-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-add-cluster-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations: testClusterAgentToleration,
					},
				},
			},
			expectedRedeploy: true,
		},
		{
			name: "test-update-cluster-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-default",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations: testClusterAgentToleration,
						},
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations: testUpdatedClusterAgentToleration,
					},
				},
			},
			expectedRedeploy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v3.ClusterConditionAgentDeployed.Message(tt.cluster, "Successfully rolled out agent")
			v3.ClusterConditionAgentDeployed.True(tt.cluster)

			doRedeploy := redeployAgent(tt.cluster, "", "", nil, nil)
			assert.Equal(t, tt.expectedRedeploy, doRedeploy)
		})
	}
}

func Test_clusterDeploy_doSync(t *testing.T) {
	type fields struct {
		systemAccountManager *systemaccount.Manager
		userManager          user.Manager
		clusters             normanv3.ClusterInterface
		clusterManager       *clustermanager.Manager
		mgmt                 *config.ManagementContext
		nodeLister           normanv3.NodeLister
		secretLister         v1.SecretLister
	}
	type args struct {
		cluster   *v3.Cluster
		connected bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test-ready-cluster-without-nodes",
			fields: fields{
				systemAccountManager: nil,
				userManager:          nil,
				clusters:             nil,
				clusterManager:       nil,
				mgmt:                 nil,
				nodeLister: &fakes.NodeListerMock{
					ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Node, error) {
						return []*v3.Node{}, nil
					},
				},
				secretLister: nil,
			},
			args: args{
				cluster: &v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-default",
					},
					Spec: v3.ClusterSpec{},
				},
				connected: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "test-disconnected-cluster",
			fields: fields{
				systemAccountManager: &systemaccount.Manager{},
				userManager:          nil,
				clusters:             nil,
				clusterManager:       &clustermanager.Manager{},
				mgmt:                 &config.ManagementContext{},
				nodeLister: &fakes.NodeListerMock{
					ListFunc: func(namespace string, selector labels.Selector) ([]*v3.Node, error) {
						return nil, errors.New("unexpected call to nodeLister.List")
					},
				},
				secretLister: nil,
			},
			args: args{
				cluster: &v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-default",
					},
					Spec: v3.ClusterSpec{},
				},
				connected: false,
			},
			wantErr: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.Equal(t, err.Error(), "cluster agent not connected")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &clusterDeploy{
				systemAccountManager: tt.fields.systemAccountManager,
				userManager:          tt.fields.userManager,
				clusters:             tt.fields.clusters,
				clusterManager:       tt.fields.clusterManager,
				mgmt:                 tt.fields.mgmt,
				nodeLister:           tt.fields.nodeLister,
				secretLister:         tt.fields.secretLister,
			}
			v3.ClusterConditionProvisioned.True(tt.args.cluster)
			if tt.args.connected {
				v3.ClusterConditionReady.True(tt.args.cluster)
			} else {
				v3.ClusterConditionReady.False(tt.args.cluster)
				v3.ClusterConditionReady.Reason(tt.args.cluster, "Disconnected")
			}
			tt.wantErr(t, cd.doSync(tt.args.cluster), fmt.Sprintf("doSync(%v)", tt.args.cluster))
		})
	}
}
