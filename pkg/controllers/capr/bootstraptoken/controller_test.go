package bootstraptoken

import (
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parseTime(t *testing.T, timeString string) time.Time {
	ret, err := time.Parse(time.RFC3339, timeString)

	require.NoError(t, err)

	return ret
}

func Test_tokenTTL(t *testing.T) {
	tests := []struct {
		name     string
		secret   *corev1.Secret
		shortTTL string
		longTTL  string
		now      time.Time
		expect   time.Duration
	}{
		{
			name: "Valid and accessed",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.BootstrapTokenLastAccessTimeAnnotation: "2026-05-07T16:01:45Z",
					},
					CreationTimestamp: metav1.Time{
						Time: parseTime(t, "2026-05-07T16:00:00Z"),
					},
				},
			},
			now:      parseTime(t, "2026-05-07T16:02:00Z"),
			shortTTL: "30s",
			longTTL:  "20m",
			expect:   15 * time.Second,
		},
		{
			name: "Valid, no access",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{
						Time: parseTime(t, "2026-05-07T16:00:00Z"),
					},
				},
			},
			now:      parseTime(t, "2026-05-07T16:02:00Z"),
			shortTTL: "30s",
			longTTL:  "20m",
			expect:   18 * time.Minute,
		},
		{
			name: "Expired by short TTL",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.BootstrapTokenLastAccessTimeAnnotation: "2026-05-07T16:01:45Z",
					},
					CreationTimestamp: metav1.Time{
						Time: parseTime(t, "2026-05-07T16:00:00Z"),
					},
				},
			},
			now:      parseTime(t, "2026-05-07T16:02:16Z"),
			shortTTL: "30s",
			longTTL:  "20m",
			expect:   -1 * time.Second,
		},
		{
			name: "Expired by long TTL",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{
						Time: parseTime(t, "2026-05-07T16:00:00Z"),
					},
				},
			},
			now:      parseTime(t, "2026-05-07T16:21:00Z"),
			shortTTL: "30s",
			longTTL:  "20m",
			expect:   -1 * time.Minute,
		},
		{
			name:     "Bad short TTL",
			shortTTL: "foobar",
			longTTL:  "20m",
			expect:   0,
		},
		{
			name:     "Bad long TTL",
			shortTTL: "30s",
			longTTL:  "foobar",
			expect:   0,
		},
		{
			name: "Bad access time",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.BootstrapTokenLastAccessTimeAnnotation: "foobar",
					},
					CreationTimestamp: metav1.Time{
						Time: parseTime(t, "2026-05-07T16:00:00Z"),
					},
				},
			},
			shortTTL: "30s",
			longTTL:  "20m",
			expect:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.SystemAgentInstallerTokenShortTTL.Set(tt.shortTTL)
			settings.SystemAgentInstallerTokenLongTTL.Set(tt.longTTL)

			timeLeft := tokenTTL(tt.secret, tt.now)

			assert.Equal(t, tt.expect, timeLeft)
		})
	}
}
