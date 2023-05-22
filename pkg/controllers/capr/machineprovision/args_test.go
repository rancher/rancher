package machineprovision

import (
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestData struct {
	metav1.ObjectMeta
}

func TestGetInstanceName(t *testing.T) {

	tests := []struct {
		name     string
		data     TestData
		expected string
	}{
		{
			name: "Single character name",
			data: TestData{
				metav1.ObjectMeta{
					Name: "a",
				},
			},
			expected: "a",
		},
		{
			name: "Period replacement",
			data: TestData{
				metav1.ObjectMeta{
					Name: "a.",
				},
			},
			expected: "a-",
		},
		{
			name: "Max length name - 63 characters",
			data: TestData{
				metav1.ObjectMeta{
					Name: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345678",
				},
			},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef012345678",
		},
		{
			name: "> Max length name - 64 characters",
			data: TestData{
				metav1.ObjectMeta{
					Name: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef012-aa23e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceName := getInstanceName(infraObject{meta: &tt.data})
			assert.Equal(t, tt.expected, instanceName)
		})
	}
}

func TestGetHostname(t *testing.T) {

	tests := []struct {
		name     string
		data     TestData
		expected string
	}{
		{
			name: "Single character name - no truncation",
			data: TestData{
				metav1.ObjectMeta{
					Name: "a",
				},
			},
			expected: "a",
		},
		{
			name: "Period replacement - no truncation",
			data: TestData{
				metav1.ObjectMeta{
					Name: "a.",
				},
			},
			expected: "a-",
		},
		{
			name: "Max length name - 63 characters",
			data: TestData{
				metav1.ObjectMeta{
					Name: "abcdef0123456789abcdef0123456789abcdef0123456789abcde0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcde0123456789",
		},
		{
			name: "> Max length name - 64 characters",
			data: TestData{
				metav1.ObjectMeta{
					Name: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef012-aa23e",
		},
		{
			name: "32 character name - limit < 32",
			data: TestData{
				metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.HostnameLengthLimitAnnotation: "24",
					},
					Name: "abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789ab-bc373",
		},
		{
			name: "32 character name - limit < minimum",
			data: TestData{
				metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.HostnameLengthLimitAnnotation: "9",
					},
					Name: "abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789",
		},
		{
			name: "32 character name - limit > maximum",
			data: TestData{
				metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.HostnameLengthLimitAnnotation: "64",
					},
					Name: "abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789",
		},
		{
			name: "64 character name - limit > maximum",
			data: TestData{
				metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.HostnameLengthLimitAnnotation: "64",
					},
					Name: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				},
			},
			expected: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef012-aa23e",
		},
		{
			name: "10 character name - limit < minimum",
			data: TestData{
				metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.HostnameLengthLimitAnnotation: "9",
					},
					Name: "abcdef0123",
				},
			},
			expected: "abcdef0123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostname := getHostname(infraObject{meta: &tt.data})
			assert.Equal(t, tt.expected, hostname)
		})
	}
}
