package nodedriver

import (
	"strings"
	"testing"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCreateFlagsJobName(t *testing.T) {
	assert.Equal(t, "nodedriver-amazonec2-getcreateflags", getCreateFlagsJobName("amazonec2"))
	assert.Equal(t, "nodedriver-custom-getcreateflags", getCreateFlagsJobName("custom"))
	assert.Equal(t, "nodedriver-veryveryveryveryveryveryveryveryveryverylongd-60348", getCreateFlagsJobName(strings.Repeat("very", 10)+"longdrivername"))
}

func TestObjectKeyIncludesDockerMachineVersion(t *testing.T) {
	l := &Lifecycle{
		dockerMachineVersion: "v0.15.0",
	}

	builtinDriver := &v32.NodeDriver{
		Spec: v32.NodeDriverSpec{
			DisplayName: "amazonec2",
			Builtin:     true,
		},
	}
	assert.Equal(t, "amazonec2-v0.15.0", l.objectKey(builtinDriver))

	customDriver := &v32.NodeDriver{
		Spec: v32.NodeDriverSpec{
			DisplayName: "custom",
			URL:         "https://example.com/driver",
			Checksum:    "abc123",
		},
	}
	key := l.objectKey(customDriver)
	assert.Contains(t, key, "custom")
	assert.Contains(t, key, "v0.15.0")
}

func TestIsJobFinished(t *testing.T) {
	// Running job without conditions or completion time
	runningJob := &batchv1.Job{}
	assert.False(t, isJobFinished(runningJob))

	// Succeeded job with completion time
	now := metav1.NewTime(time.Now())
	succeededJobWithTime := &batchv1.Job{
		Status: batchv1.JobStatus{
			CompletionTime: &now,
		},
	}
	assert.True(t, isJobFinished(succeededJobWithTime))

	// Succeeded job with JobComplete condition
	succeededJobWithCond := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	assert.True(t, isJobFinished(succeededJobWithCond))

	// Failed job with JobFailed condition
	failedJob := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	assert.True(t, isJobFinished(failedJob))
}

func TestGetDriverDownloadURLFallback(t *testing.T) {
	driver := &v32.NodeDriver{
		Spec: v32.NodeDriverSpec{
			DisplayName: "linode",
			URL:         "https://example.com/linode.zip",
			Checksum:    "1a8336f66",
		},
	}
	url, hash, err := getDriverDownloadURL(driver)
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/linode.zip", url)
	assert.Equal(t, "1a8336f66", hash)
}

