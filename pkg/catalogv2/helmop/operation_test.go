package helmop

import (
	"fmt"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	commands Commands
	expected map[string][]byte
	failMsg  string
}

type createPodTestCase struct {
	name          string
	operation     Operations
	expected      *corev1.Pod
	failMsg       string
	secretData    map[string][]byte
	kustomize     bool
	imageOverride string
	tolerations   []corev1.Toleration
}

type mergeTolerationsTestCase struct {
	name           string
	tolerations    []corev1.Toleration
	newTolerations []corev1.Toleration
	expected       []corev1.Toleration
}

func Test_Render(t *testing.T) {
	asserts := assert.New(t)

	testCases := []testCase{
		{
			commands: Commands{
				Command{
					Operation:   "upgrade",
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   true,
					ReleaseName: "test1",
				},
			},
			expected: map[string][]byte{
				"operation000":          []byte(strings.Join([]string{"upgrade", "--post-renderer=/home/shell/kustomize.sh", "test1", "/home/shell/helm-run/test-chart-v1.1.0.tgz"}, "\x00")),
				"kustomization000.yaml": []byte(fmt.Sprintf(kustomization, "000")),
				"transform000.yaml":     []byte(fmt.Sprintf(transform, "test1")),
				"test-chart-v1.1.0.tgz": []byte("test-chart"),
			},
			failMsg: "kustomize enabled test case failed",
		},
		{
			commands: Commands{
				Command{
					Operation:   "upgrade",
					ValuesFile:  "values-test-chart-v1.1.0.yaml",
					Values:      []byte("{\"a\":\"a\"}"),
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   false,
					ReleaseName: "test2",
				},
			},
			expected: map[string][]byte{
				"operation000":                  []byte(strings.Join([]string{"upgrade", "--values=/home/shell/helm/values-test-chart-v1.1.0.yaml", "test2", "/home/shell/helm/test-chart-v1.1.0.tgz"}, "\x00")),
				"test-chart-v1.1.0.tgz":         []byte("test-chart"),
				"values-test-chart-v1.1.0.yaml": []byte("{\"a\":\"a\"}"),
			},
			failMsg: "values yaml test case failed",
		},
		{
			commands: Commands{
				Command{
					Operation:   "upgrade",
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   false,
					ReleaseName: "test3",
				},
			},
			expected: map[string][]byte{
				"operation000":          []byte(strings.Join([]string{"upgrade", "test3", "/home/shell/helm/test-chart-v1.1.0.tgz"}, "\x00")),
				"test-chart-v1.1.0.tgz": []byte("test-chart"),
			},
			failMsg: "default test case failed",
		},
		{
			commands: Commands{
				Command{
					Operation:   "install",
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   false,
					ReleaseName: "test4",
				},
			},
			expected: map[string][]byte{
				"operation000":          []byte(strings.Join([]string{"install", "test4", "/home/shell/helm/test-chart-v1.1.0.tgz"}, "\x00")),
				"test-chart-v1.1.0.tgz": []byte("test-chart"),
			},
			failMsg: "install test case failed",
		},
		{
			commands: Commands{
				Command{
					Operation:   "uninstall",
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   false,
					ReleaseName: "test5",
				},
			},
			expected: map[string][]byte{
				"operation000":          []byte(strings.Join([]string{"uninstall", "test5", "/home/shell/helm/test-chart-v1.1.0.tgz"}, "\x00")),
				"test-chart-v1.1.0.tgz": []byte("test-chart"),
			},
			failMsg: "uninstall test case failed",
		},
		{
			commands: Commands{
				Command{
					Operation:   "upgrade",
					ChartFile:   "test-chart-v1.1.0.tgz",
					Chart:       []byte("test-chart"),
					Kustomize:   true,
					ReleaseName: "test6",
					ArgObjects: []interface{}{types.ChartInstallAction{
						OperationTolerations: []corev1.Toleration{
							{
								Key:      "foo",
								Operator: "equals",
								Value:    "bar",
								Effect:   "NoSchedule",
							},
							{
								Key:      "foo2",
								Operator: "equals",
								Value:    "bar2",
								Effect:   "NoSchedule",
							}},
					}},
				},
			},
			expected: map[string][]byte{
				"operation000":          []byte(strings.Join([]string{"upgrade", "--post-renderer=/home/shell/kustomize.sh", "test6", "/home/shell/helm-run/test-chart-v1.1.0.tgz"}, "\x00")),
				"kustomization000.yaml": []byte(fmt.Sprintf(kustomization, "000")),
				"transform000.yaml":     []byte(fmt.Sprintf(transform, "test6")),
				"test-chart-v1.1.0.tgz": []byte("test-chart"),
			},
			failMsg: "operation toleration test case failed",
		},
	}

	for _, testCase := range testCases {
		actual, err := testCase.commands.Render()
		asserts.Nil(err, "error encountered: %v", err)
		asserts.Equal(testCase.expected, actual, testCase.failMsg)
	}
}

// Test_CreatePod function to run tests for the createPod method
func Test_CreatePod(t *testing.T) {
	asserts := assert.New(t)
	defaultTolerations := []corev1.Toleration{
		{
			Key:      "cattle.io/os",
			Operator: corev1.TolerationOpEqual,
			Value:    "linux",
			Effect:   "NoSchedule",
		},
		{
			Key:      "node-role.kubernetes.io/controlplane",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   "NoSchedule",
		},
		{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   "NoSchedule",
		},
		{
			Key:      "node-role.kubernetes.io/etcd",
			Operator: corev1.TolerationOpExists,
			Effect:   "NoExecute",
		},
		{
			Key:      "node.cloudprovider.kubernetes.io/uninitialized",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   "NoSchedule",
		},
	}
	testCases := []createPodTestCase{
		{
			name: "Created pod should have custom toleration",
			operation: Operations{
				namespace: "test-ns",
			},
			expected: &corev1.Pod{
				Spec: corev1.PodSpec{Tolerations: append(defaultTolerations, corev1.Toleration{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				})},
			},
			tolerations: []corev1.Toleration{{
				Key:      "foo",
				Operator: "Equals",
				Value:    "bar",
				Effect:   "NoSchedule",
			}},
			failMsg: "Pod should be created with toleration",
		},
		{
			name: "Created pod should have custom tolerations",
			operation: Operations{
				namespace: "test-ns",
			},
			expected: &corev1.Pod{
				Spec: corev1.PodSpec{Tolerations: append(defaultTolerations, corev1.Toleration{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				}, corev1.Toleration{
					Key:   "foo2",
					Value: "bar2",
				})},
			},
			tolerations: []corev1.Toleration{
				{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				},
				{
					Key:   "foo2",
					Value: "bar2",
				}},
			failMsg: "Pod should be created with all custom tolerations",
		},
		{
			name: "Created pod should have only the default tolerations",
			operation: Operations{
				namespace: "test-ns",
			},
			expected: &corev1.Pod{
				Spec: corev1.PodSpec{Tolerations: defaultTolerations},
			},
			tolerations: nil,
			failMsg:     "Pod should be created with only the default tolerations",
		},
	}
	for _, testCase := range testCases {
		actual, _ := testCase.operation.createPod(testCase.secretData, testCase.kustomize, testCase.imageOverride, testCase.tolerations)
		asserts.ElementsMatch(testCase.expected.Spec.Tolerations, actual.Spec.Tolerations, testCase.failMsg)
	}
}

func Test_mergeTolerations(t *testing.T) {
	asserts := assert.New(t)
	testCases := []mergeTolerationsTestCase{
		{
			name: "Shouldn't duplicate tolerations",
			tolerations: []corev1.Toleration{
				{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				},
				{
					Key:   "foo2",
					Value: "bar2",
				},
			},
			newTolerations: []corev1.Toleration{
				{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				},
				{
					Key:   "foo3",
					Value: "bar3",
				},
				{
					Key:   "foo2",
					Value: "bar20",
				},
			},
			expected: []corev1.Toleration{
				{
					Key:      "foo",
					Operator: "Equals",
					Value:    "bar",
					Effect:   "NoSchedule",
				},
				{
					Key:   "foo2",
					Value: "bar2",
				},
				{
					Key:   "foo3",
					Value: "bar3",
				},
				{
					Key:   "foo2",
					Value: "bar20",
				},
			},
		},
	}
	for _, t := range testCases {
		resp := mergeTolerations(t.tolerations, t.newTolerations)
		asserts.ElementsMatch(resp, t.expected, t.name)
	}
}
