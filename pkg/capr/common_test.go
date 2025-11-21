package capr

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
							APIVersion: "cluster.x-k8s.io/v1beta1",
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
							APIVersion: "cluster.x-k8s.io/v1beta1",
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

func TestCompressInterface(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{
			name:  "int",
			value: &[]int{1}[0],
		},
		{
			name:  "string",
			value: &[]string{"test"}[0],
		},
		{
			name: "struct",
			value: &struct {
				TestInt    int
				TestString string
			}{
				TestInt:    1,
				TestString: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompressInterface(tt.value)
			assert.Nil(t, err)
			assert.True(t, result != "")

			target := reflect.New(reflect.ValueOf(tt.value).Elem().Type()).Interface()

			err = decompressInterface(result, target)
			assert.Nil(t, err)
			assert.Equal(t, tt.value, target)
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

func TestParseSnapshotClusterSpecOrError(t *testing.T) {
	// Constants assumed from the implementation context
	const metaKey = "provisioning-cluster-spec"

	// Helper to create the outer metadata base64 string
	createMetadata := func(data map[string]string) string {
		jsonBytes, err := json.Marshal(data)
		require.NoError(t, err)
		return base64.StdEncoding.EncodeToString(jsonBytes)
	}

	// Valid Data Setup
	validSpec := &provv1.ClusterSpec{
		KubernetesVersion: "v1.26.0",
	}
	validCompressed, err := CompressInterface(validSpec)
	require.NoError(t, err)

	tests := []struct {
		name          string
		snapshot      *rkev1.ETCDSnapshot
		expectedSpec  *provv1.ClusterSpec
		expectedError string
	}{
		{
			name:          "nil snapshot",
			snapshot:      nil,
			expectedSpec:  nil,
			expectedError: "snapshot was nil",
		},
		{
			name: "empty metadata string",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: "",
				},
			},
			expectedSpec:  nil,
			expectedError: "metadata map is empty",
		},
		{
			name: "outer layer invalid base64",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: "!!! not base64 !!!",
				},
			},
			expectedSpec:  nil,
			expectedError: "base64 decode failed",
		},
		{
			name: "outer layer valid base64 but invalid json",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: base64.StdEncoding.EncodeToString([]byte(`{invalid-json`)),
				},
			},
			expectedSpec:  nil,
			expectedError: "JSON unmarshal failed",
		},
		{
			name: "outer layer json missing required key",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						"wrong-key": "some-value",
					}),
				},
			},
			expectedSpec:  nil,
			expectedError: "key not found or empty",
		},
		{
			name: "outer layer json has key but empty value",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: "   ",
					}),
				},
			},
			expectedSpec:  nil,
			expectedError: "key not found or empty",
		},
		{
			name: "inner layer invalid base64 (payload corruption)",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: "!!! not inner base64 !!!",
					}),
				},
			},
			expectedSpec: nil,
			// Matches the wrap: "reading snapshot metadata into ClusterSpec: ... base64 decode failed"
			expectedError: "reading snapshot metadata into ClusterSpec",
		},
		{
			name: "inner layer valid base64 but not gzip (gzip header missing)",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						// Valid base64, but just a raw string, not gzipped
						metaKey: base64.StdEncoding.EncodeToString([]byte("not-gzipped-data")),
					}),
				},
			},
			expectedSpec: nil,
			// Matches the wrap in decompressInterface: "gzip decompress failed"
			expectedError: "gzip decompress failed",
		},
		{
			name: "success valid snapshot",
			snapshot: &rkev1.ETCDSnapshot{
				SnapshotFile: rkev1.ETCDSnapshotFile{
					Metadata: createMetadata(map[string]string{
						metaKey: validCompressed,
					}),
				},
			},
			expectedSpec:  validSpec,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSnapshotClusterSpecOrError(tt.snapshot)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSpec, result)
			}
		})
	}
}
