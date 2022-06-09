package helmop

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	commands Commands
	expected map[string][]byte
	failMsg  string
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
	}
	for _, testCase := range testCases {
		actual, err := testCase.commands.Render()
		asserts.Nil(err, "error encountered: %v", err)
		asserts.Equal(testCase.expected, actual, testCase.failMsg)
	}
}
