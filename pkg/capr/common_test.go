package capr

import (
	"testing"

	"github.com/pkg/errors"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Implements ClusterCache
type test struct {
	name        string
	expected    *capi.Cluster
	expectedErr error
	obj         runtime.Object
}

func TestFindCAPIClusterFromLabel(t *testing.T) {
	tests := []test{
		{
			name:        "nil",
			expected:    nil,
			expectedErr: errNilObject,
			obj:         nil,
		},
		{
			name:        "missing label",
			expected:    nil,
			expectedErr: errors.New("cluster.x-k8s.io/cluster-name label not present on testObject: testNamespace/testName"),
			obj: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
					Labels:    map[string]string{},
				},
				TypeMeta: metav1.TypeMeta{Kind: "testObject"},
			},
		},
		{
			name:     "missing cluster",
			expected: nil,
			obj: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": "testCluster",
					},
				},
			},
		},
		{
			name:        "success",
			expected:    &capi.Cluster{},
			expectedErr: nil,
			obj: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name": "testCluster",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			capiCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
			capiCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tt.expected, nil).MaxTimes(1)
			cluster, err := GetCAPIClusterFromLabel(tt.obj, capiCache)
			if err == nil {
				assert.Nil(t, tt.expectedErr)
			} else if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Fail(t, "expected err to be nil, was actually %s", err)
			}
			assert.Equal(t, tt.expected, cluster)
		})
	}
}

func TestFindOwnerCAPICluster(t *testing.T) {
	tests := []test{
		{
			name:        "nil",
			expected:    nil,
			expectedErr: errNilObject,
			obj:         nil,
		},
		{
			name:        "no owner",
			expected:    nil,
			expectedErr: ErrNoMatchingControllerOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
				},
			},
		},
		{
			name:        "no controller",
			expected:    nil,
			expectedErr: ErrNoMatchingControllerOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "nil",
						},
					},
				},
			},
		},
		{
			name:        "owner wrong kind",
			expected:    nil,
			expectedErr: ErrNoMatchingControllerOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "nil",
							APIVersion: "cluster.x-k8s.io/v1beta2",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
		{
			name:        "owner wrong api version",
			expected:    nil,
			expectedErr: ErrNoMatchingControllerOwnerRef,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "Cluster",
							APIVersion: "nil",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
		{
			name:        "success",
			expected:    nil,
			expectedErr: nil,
			obj: &rkev1.RKECluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "RKECluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testcluster",
					Namespace: "testnamespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "Cluster",
							APIVersion: "cluster.x-k8s.io/v1beta2",
							Controller: &[]bool{true}[0],
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			capiCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
			capiCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tt.expected, tt.expectedErr).MaxTimes(1)
			cluster, err := GetOwnerCAPICluster(tt.obj, capiCache)
			if err == nil {
				assert.Nil(t, tt.expectedErr)
			} else if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Fail(t, "expected err to be nil, was actually %s", err)
			}
			assert.Equal(t, tt.expected, cluster)
		})
	}
}

func TestSafeConcatName(t *testing.T) {

	testcase := []struct {
		name           string
		input          []string
		expectedOutput string
		maxLength      int
	}{
		{
			name:           "max k8s name shortening",
			input:          []string{"very", "long", "name", "to", "test", "shortening", "behavior", "this", "should", "exceed", "max", "k8s", "name", "length"},
			expectedOutput: "very-long-name-to-test-shortening-behavior-this-should-ex-e8118",
			maxLength:      63,
		},
		{
			name:           "max helm release name shortening",
			maxLength:      53,
			input:          []string{"long", "cluster", "name", "testing", "managed", "system-upgrade", "controller", "fleet", "agent"},
			expectedOutput: "long-cluster-name-testing-managed-system-upgrad-0beef",
		},
		{
			name:           "max length smaller than hash size should concat and shorten but not hash",
			maxLength:      3,
			input:          []string{"this", "will", "not", "be", "hashed"},
			expectedOutput: "thi",
		},
		{
			name:           "concat but not shorten",
			maxLength:      90,
			input:          []string{"simple", "concat", "no", "hash", "needed"},
			expectedOutput: "simple-concat-no-hash-needed",
		},
		{
			name:           "no max length, no output",
			maxLength:      0,
			input:          []string{"input"},
			expectedOutput: "",
		},
		{
			name:           "input equal to hash length should return hash without leading '-'",
			maxLength:      6,
			input:          []string{"input", "s"},
			expectedOutput: "deab5",
		},
		{
			name:           "avoid special characters",
			maxLength:      8,
			input:          []string{"a", "&", "b", "=", "c"},
			expectedOutput: "a-359087",
		},
	}

	for _, tc := range testcase {
		t.Run(tc.name, func(t *testing.T) {
			out := SafeConcatName(tc.maxLength, tc.input...)
			if len(out) > tc.maxLength || out != tc.expectedOutput {
				t.Fail()
				t.Logf("expected output %s with length of %d, got %s with length of %d", tc.expectedOutput, len(tc.expectedOutput), out, len(out))
			}
			t.Log(out)
		})
	}
}

func TestFormatWindowsEnvVar(t *testing.T) {
	tests := []struct {
		Name           string
		EnvVar         corev1.EnvVar
		IsPlanVar      bool
		ExpectedString string
	}{
		{
			Name: "Basic String",
			EnvVar: corev1.EnvVar{
				Name:  "BASIC_STRING",
				Value: "ABC123",
			},
			IsPlanVar:      false,
			ExpectedString: "$env:BASIC_STRING=\"ABC123\"",
		},
		{
			Name: "Basic Bool",
			EnvVar: corev1.EnvVar{
				Name:  "BASIC_BOOL",
				Value: "true",
			},
			IsPlanVar:      false,
			ExpectedString: "$env:BASIC_BOOL=$true",
		},
		{
			Name: "Basic Plan String",
			EnvVar: corev1.EnvVar{
				Name:  "PLAN_STRING",
				Value: "VALUE",
			},
			IsPlanVar:      true,
			ExpectedString: "PLAN_STRING=VALUE",
		},
		{
			Name: "Basic Plan Bool",
			EnvVar: corev1.EnvVar{
				Name:  "PLAN_BOOL",
				Value: "true",
			},
			IsPlanVar:      true,
			ExpectedString: "PLAN_BOOL=true",
		},
		{
			Name: "Plan Name Mistakenly Includes $env:",
			EnvVar: corev1.EnvVar{
				Name:  "$env:PLAN_BOOL",
				Value: "true",
			},
			IsPlanVar:      true,
			ExpectedString: "PLAN_BOOL=true",
		},
		{
			Name: "Plan Bool Mistakenly Includes $",
			EnvVar: corev1.EnvVar{
				Name:  "PLAN_BOOL",
				Value: "$true",
			},
			IsPlanVar:      true,
			ExpectedString: "PLAN_BOOL=true",
		},
		{
			Name: "Non-Plan String Value Includes $",
			EnvVar: corev1.EnvVar{
				Name:  "PLAN_BOOL",
				Value: "\"$true\"",
			},
			IsPlanVar:      false,
			ExpectedString: "$env:PLAN_BOOL=\"$true\"",
		},
		{
			Name: "Plan String Value Includes $",
			EnvVar: corev1.EnvVar{
				Name:  "PLAN_BOOL",
				Value: "\"$true\"",
			},
			IsPlanVar:      true,
			ExpectedString: "PLAN_BOOL=$true",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			out := FormatWindowsEnvVar(tc.EnvVar, tc.IsPlanVar)
			if out != tc.ExpectedString {
				t.Fatalf("Expected %s, got %s", tc.ExpectedString, out)
			}
		})
	}
}

func TestAutoscalingEnabledByCAPI(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *capi.Cluster
		mds      []*capi.MachineDeployment
		expected bool
	}{
		{
			name: "autoscaling enabled with single machine deployment",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{ClusterAutoscalerEnabledAnnotation: "true"},
				},
			},
			mds: []*capi.MachineDeployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-md",
						Namespace: "default",
						Annotations: map[string]string{
							capi.AutoscalerMinSizeAnnotation: "2",
							capi.AutoscalerMaxSizeAnnotation: "10",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "autoscaling enabled with multiple machine deployments where one has autoscaling annotations",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{ClusterAutoscalerEnabledAnnotation: "true"},
				},
			},
			mds: []*capi.MachineDeployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-md-1",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-md-2",
						Namespace: "default",
						Annotations: map[string]string{
							capi.AutoscalerMinSizeAnnotation: "2",
							capi.AutoscalerMaxSizeAnnotation: "10",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "autoscaling disabled when cluster annotation is not set",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
			},
			mds: []*capi.MachineDeployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-md",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
				},
			},
			expected: false,
		},
		{
			name: "autoscaling disabled when no machine deployments have autoscaling annotations",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{ClusterAutoscalerEnabledAnnotation: "true"},
				},
			},
			mds: []*capi.MachineDeployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-md-1",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-md-2",
						Namespace:   "default",
						Annotations: map[string]string{capi.AutoscalerMinSizeAnnotation: "2"},
					},
				},
			},
			expected: false,
		},
		{
			name: "autoscaling disabled when machine deployments are empty",
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "default",
					Annotations: map[string]string{ClusterAutoscalerEnabledAnnotation: "true"},
				},
			},
			mds:      []*capi.MachineDeployment{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AutoscalerEnabledByCAPI(tt.cluster, tt.mds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProvisioningClusterFromCAPICluster(t *testing.T) {
	tests := []struct {
		name        string
		capiCluster *capi.Cluster
		cluster     *provv1.Cluster
		expectedErr error
	}{
		{
			name: "HappyPath_Existing_Cluster",
			capiCluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Name:       "foo",
					}},
				},
			},
			cluster: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expectedErr: nil,
		},
		{
			name: "SadPath_No_Cluster",
			capiCluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Name:       "foo",
					}},
				},
			},
			expectedErr: errors.New("not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			capiCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
			capiCache.EXPECT().Get(tt.capiCluster.Namespace, tt.capiCluster.Name).Return(tt.cluster, tt.expectedErr).MaxTimes(1)

			cluster, err := GetProvisioningClusterFromCAPICluster(tt.capiCluster, capiCache)

			if err == nil {
				assert.Nil(t, tt.expectedErr)
			} else if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.Fail(t, "expected err to be nil, was actually %s", err)
			}
			assert.Equal(t, tt.cluster, cluster)
		})
	}
}

func TestSetCAPIResourceCondition(t *testing.T) {
	tests := []struct {
		name      string
		obj       CAPIConditionSetter
		condition metav1.Condition
		// expectedV1Beta2Condition is checked against the object's status.conditions
		expectedV1Beta2Status metav1.ConditionStatus
		expectedV1Beta1Status corev1.ConditionStatus
		expectedReason        string
		expectedMessage       string
	}{
		{
			name: "set Reconciled condition to true on machine",
			obj:  &capi.Machine{},
			condition: metav1.Condition{
				Type:   string(Reconciled),
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			},
			expectedV1Beta2Status: metav1.ConditionTrue,
			expectedV1Beta1Status: corev1.ConditionTrue,
			expectedReason:        "Reconciled",
		},
		{
			name: "set Reconciled condition to unknown with message on machine",
			obj:  &capi.Machine{},
			condition: metav1.Condition{
				Type:    string(Reconciled),
				Status:  metav1.ConditionUnknown,
				Reason:  "Waiting",
				Message: "waiting for something",
			},
			expectedV1Beta2Status: metav1.ConditionUnknown,
			expectedV1Beta1Status: corev1.ConditionUnknown,
			expectedReason:        "Waiting",
			expectedMessage:       "waiting for something",
		},
		{
			name: "set PlanApplied condition to false with error on machine",
			obj:  &capi.Machine{},
			condition: metav1.Condition{
				Type:    string(PlanApplied),
				Status:  metav1.ConditionFalse,
				Reason:  "Error",
				Message: "error applying plan",
			},
			expectedV1Beta2Status: metav1.ConditionFalse,
			expectedV1Beta1Status: corev1.ConditionFalse,
			expectedReason:        "Error",
			expectedMessage:       "error applying plan",
		},
		{
			name: "set PlanApplied condition to true on machine",
			obj:  &capi.Machine{},
			condition: metav1.Condition{
				Type:   string(PlanApplied),
				Status: metav1.ConditionTrue,
				Reason: "PlanApplied",
			},
			expectedV1Beta2Status: metav1.ConditionTrue,
			expectedV1Beta1Status: corev1.ConditionTrue,
			expectedReason:        "PlanApplied",
		},
		{
			name: "update existing condition on machine",
			obj: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(Reconciled),
							Status: metav1.ConditionUnknown,
							Reason: "Waiting",
						},
					},
					Deprecated: &capi.MachineDeprecatedStatus{
						V1Beta1: &capi.MachineV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{
									Type:   capi.ConditionType(string(Reconciled)),
									Status: corev1.ConditionUnknown,
									Reason: "Waiting",
								},
							},
						},
					},
				},
			},
			condition: metav1.Condition{
				Type:   string(Reconciled),
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			},
			expectedV1Beta2Status: metav1.ConditionTrue,
			expectedV1Beta1Status: corev1.ConditionTrue,
			expectedReason:        "Reconciled",
		},
		{
			name: "set condition to true on cluster",
			obj:  &capi.Cluster{},
			condition: metav1.Condition{
				Type:   string(Reconciled),
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			},
			expectedV1Beta2Status: metav1.ConditionTrue,
			expectedV1Beta1Status: corev1.ConditionTrue,
			expectedReason:        "Reconciled",
		},
		{
			name: "set condition to false on cluster",
			obj:  &capi.Cluster{},
			condition: metav1.Condition{
				Type:    capi.ClusterControlPlaneInitializedCondition,
				Status:  metav1.ConditionFalse,
				Reason:  capi.ClusterControlPlaneNotInitializedReason,
				Message: "Waiting for control plane provider to indicate the control plane has been initialized",
			},
			expectedV1Beta2Status: metav1.ConditionFalse,
			expectedV1Beta1Status: corev1.ConditionFalse,
			expectedReason:        capi.ClusterControlPlaneNotInitializedReason,
			expectedMessage:       "Waiting for control plane provider to indicate the control plane has been initialized",
		},
		{
			name: "update existing condition on cluster",
			obj: &capi.Cluster{
				Status: capi.ClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(Reconciled),
							Status: metav1.ConditionUnknown,
							Reason: "Waiting",
						},
					},
					Deprecated: &capi.ClusterDeprecatedStatus{
						V1Beta1: &capi.ClusterV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{
									Type:   capi.ConditionType(string(Reconciled)),
									Status: corev1.ConditionUnknown,
									Reason: "Waiting",
								},
							},
						},
					},
				},
			},
			condition: metav1.Condition{
				Type:   string(Reconciled),
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			},
			expectedV1Beta2Status: metav1.ConditionTrue,
			expectedV1Beta1Status: corev1.ConditionTrue,
			expectedReason:        "Reconciled",
		},
		{
			name: "nil object does not panic",
			obj:  nil,
			condition: metav1.Condition{
				Type:   string(Reconciled),
				Status: metav1.ConditionTrue,
				Reason: "Reconciled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCAPIResourceCondition(tt.obj, tt.condition)
			if tt.obj == nil {
				return
			}

			// Verify v1beta2 condition was set
			var v1beta2Found bool
			for _, c := range tt.obj.GetConditions() {
				if c.Type == tt.condition.Type {
					v1beta2Found = true
					assert.Equal(t, tt.expectedV1Beta2Status, c.Status)
					assert.Equal(t, tt.expectedReason, c.Reason)
					assert.Equal(t, tt.expectedMessage, c.Message)
					break
				}
			}
			assert.True(t, v1beta2Found, "v1beta2 condition should be set on status.conditions")

			// Verify v1beta1 deprecated condition was set
			v1beta1Conditions := tt.obj.GetV1Beta1Conditions()
			var v1beta1Found bool
			for _, c := range v1beta1Conditions {
				if string(c.Type) == tt.condition.Type {
					v1beta1Found = true
					assert.Equal(t, tt.expectedV1Beta1Status, c.Status)
					assert.Equal(t, tt.expectedReason, c.Reason)
					assert.Equal(t, tt.expectedMessage, c.Message)
					break
				}
			}
			assert.True(t, v1beta1Found, "v1beta1 condition should be set on status.deprecated.v1beta1.conditions")
		})
	}
}
