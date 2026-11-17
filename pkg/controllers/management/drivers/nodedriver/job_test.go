package nodedriver

import (
	"strings"
	"testing"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestGetJobStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobName := "nodedriver-amazonec2-getcreateflags"
	expectedHash := "hash123"

	t.Run("NotFound", func(t *testing.T) {
		jobCache := fake.NewMockCacheInterface[*batchv1.Job](ctrl)
		jobCache.EXPECT().Get(namespace.System, jobName).Return(nil, apierrors.NewNotFound(schema.GroupResource{Group: "batch", Resource: "jobs"}, jobName))

		l := &Lifecycle{jobCache: jobCache}
		status, payload, err := l.getJobStatus(jobName, expectedHash)
		require.NoError(t, err)
		assert.Equal(t, jobStatusNotFound, status)
		assert.Nil(t, payload)
	})

	t.Run("Stale (hash mismatch)", func(t *testing.T) {
		jobCache := fake.NewMockCacheInterface[*batchv1.Job](ctrl)
		staleJob := &batchv1.Job{
			Name: jobName,
			Annotations: map[string]string{
				driverHashAnnotation: "old-hash",
			},
		}
		jobCache.EXPECT().Get(namespace.System, jobName).Return(staleJob, nil)

		l := &Lifecycle{jobCache: jobCache}
		status, payload, err := l.getJobStatus(jobName, expectedHash)
		require.NoError(t, err)
		assert.Equal(t, jobStatusStale, status)
		assert.Nil(t, payload)
	})

	t.Run("Running (no conditions)", func(t *testing.T) {
		jobCache := fake.NewMockCacheInterface[*batchv1.Job](ctrl)
		runningJob := &batchv1.Job{
			Name: jobName,
			Annotations: map[string]string{
				driverHashAnnotation: expectedHash,
			},
		}
		jobCache.EXPECT().Get(namespace.System, jobName).Return(runningJob, nil)

		l := &Lifecycle{jobCache: jobCache}
		status, payload, err := l.getJobStatus(jobName, expectedHash)
		require.NoError(t, err)
		assert.Equal(t, jobStatusRunning, status)
		assert.Nil(t, payload)
	})

	t.Run("Failed with container termination message", func(t *testing.T) {
		jobCache := fake.NewMockCacheInterface[*batchv1.Job](ctrl)
		podCache := fake.NewMockCacheInterface[*corev1.Pod](ctrl)

		failedJob := &batchv1.Job{
			Name:      jobName,
			Namespace: namespace.System,
			Annotations: map[string]string{
				driverHashAnnotation: expectedHash,
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		jobCache.EXPECT().Get(namespace.System, jobName).Return(failedJob, nil)

		failedPod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "get-flags",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
								Message:  "failed to download driver: 404 Not Found",
							},
						},
					},
				},
			},
		}
		podCache.EXPECT().List(namespace.System, gomock.Any()).Return([]*corev1.Pod{failedPod}, nil)

		l := &Lifecycle{
			jobCache: jobCache,
			podCache: podCache,
		}
		status, payload, err := l.getJobStatus(jobName, expectedHash)
		require.NoError(t, err)
		assert.Equal(t, jobStatusFailed, status)
		assert.Equal(t, []byte("failed to download driver: 404 Not Found"), payload)
	})

	t.Run("Failed with fallback condition message", func(t *testing.T) {
		jobCache := fake.NewMockCacheInterface[*batchv1.Job](ctrl)
		podCache := fake.NewMockCacheInterface[*corev1.Pod](ctrl)

		failedJob := &batchv1.Job{
			Name:      jobName,
			Namespace: namespace.System,
			Annotations: map[string]string{
				driverHashAnnotation: expectedHash,
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:    batchv1.JobFailed,
						Status:  corev1.ConditionTrue,
						Message: "Job has reached the specified backoff limit",
					},
				},
			},
		}
		jobCache.EXPECT().Get(namespace.System, jobName).Return(failedJob, nil)
		podCache.EXPECT().List(namespace.System, gomock.Any()).Return([]*corev1.Pod{}, nil)

		l := &Lifecycle{
			jobCache: jobCache,
			podCache: podCache,
		}
		status, payload, err := l.getJobStatus(jobName, expectedHash)
		require.NoError(t, err)
		assert.Equal(t, jobStatusFailed, status)
		assert.Equal(t, []byte("Job has reached the specified backoff limit"), payload)
	})
}
