package capr

import (
	"reflect"
	"testing"

	"github.com/pkg/errors"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
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

			err = DecompressInterface(result, target)
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
