package nodedriver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/name"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// JobLabelDriver is the label key for the driver name on get-flags Jobs
	JobLabelDriver = "nodedriver.cattle.io/driver"
	// JobLabelPurpose identifies this as a get-create-flags Job
	JobLabelPurpose = "nodedriver.cattle.io/purpose"
	// JobPurposeGetFlags is the value for JobLabelPurpose
	JobPurposeGetFlags = "rancher-machine-get-create-flags"

	getFlagsInitContainerName = "get-flags"
	getFlagsContainerName     = "print-flags"
	getFlagsVolumeName        = "output"
	getFlagsOutputPath        = "/output/flags.json"

	driverHashAnnotation = "nodedriver.cattle.io/hash"
)

// getDriverDownloadURL checks for a local version of the driver to download for air-gapped installs.
// If no local version is found or CATTLE_DEV_MODE is set, then the URL from the node driver is returned.
func getDriverDownloadURL(nd *v32.NodeDriver) (string, string, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	driverName := nd.Spec.DisplayName
	if driverName == "" {
		driverName = nd.Name
	}
	if !strings.HasPrefix(driverName, drivers.DockerMachineDriverPrefix) {
		driverName = drivers.DockerMachineDriverPrefix + driverName
	}

	path := filepath.Join(settings.UIDashboardPath.Get(), "assets", driverName)
	if _, err := os.Stat(path); err != nil {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	hash, err := hashFile(path)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/assets/%s", settings.ServerURL.Get(), driverName), hash, nil
}

func hashFile(path string) (string, error) {
	hasher := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type jobStatus string

const (
	// jobStatusNotFound indicates that no discovery Job exists for the driver.
	jobStatusNotFound jobStatus = "NotFound"
	// jobStatusStale indicates that a discovery Job exists, but was created for an older configuration hash.
	jobStatusStale jobStatus = "Stale"
	// jobStatusRunning indicates that a discovery Job matching the current configuration is still in progress.
	jobStatusRunning jobStatus = "Running"
	// jobStatusSucceeded indicates that the discovery Job has completed successfully and its output is ready.
	jobStatusSucceeded jobStatus = "Succeeded"
	// jobStatusFailed indicates that the discovery Job failed (e.g. init or regular container exited with an error).
	jobStatusFailed jobStatus = "Failed"
)

func getCreateFlagsJobName(driverName string) string {
	return name.SafeConcatName("nodedriver", driverName, "getcreateflags")
}

func isJobFinished(job *batchv1.Job) bool {
	if job.Status.CompletionTime != nil {
		return true
	}
	for _, cond := range job.Status.Conditions {
		if (cond.Type == batchv1.JobComplete || cond.Type == batchv1.JobFailed) && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// createGetFlagsJob creates a Job that runs get-create-flags for the given driver
func (l *Lifecycle) createGetFlagsJob(driverName, expectedHash, url, hash string) (*batchv1.Job, error) {
	jobName := getCreateFlagsJobName(driverName)

	jobLabels := map[string]string{
		JobLabelDriver:  driverName,
		JobLabelPurpose: JobPurposeGetFlags,
	}

	jobAnnotations := map[string]string{
		driverHashAnnotation: expectedHash,
	}

	getCreateFlagsCmd := []string{
		"get-create-flags",
		fmt.Sprintf("--driver=%s", driverName),
		fmt.Sprintf("--output=%s", getFlagsOutputPath),
	}

	// For non-builtin drivers, add download URL and hash
	if url != "" && hash != "" {
		getCreateFlagsCmd = append(getCreateFlagsCmd,
			fmt.Sprintf("--driver-download-url=%s", url),
			fmt.Sprintf("--driver-hash=%s", hash),
		)
	}

	outputVolume := corev1.Volume{
		Name:     getFlagsVolumeName,
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}

	outputVolumeMount := corev1.VolumeMount{
		Name:      getFlagsVolumeName,
		MountPath: "/output",
	}

	image := settings.PrefixPrivateRegistry(settings.MachineProvisionImage.Get())
	imagePullPolicy := settings.GetMachineProvisionImagePullPolicy()
	job := &batchv1.Job{
		Name:        jobName,
		Namespace:   namespace.System,
		Labels:      jobLabels,
		Annotations: jobAnnotations,
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32(2)), // 3 max attempts
			Template: corev1.PodTemplateSpec{
				Labels: jobLabels,
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       []corev1.Volume{outputVolume},
					// First run "rancher-machine get-create-flags ..." and write the output to a file inside a shared volume
					// It uses a init container to ensure it completed before trying to access the output file
					InitContainers: []corev1.Container{
						{
							Name: getFlagsInitContainerName,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  new(int64(1000)),
								RunAsGroup: new(int64(1000)),
							},
							Image:                    image,
							ImagePullPolicy:          imagePullPolicy,
							Args:                     getCreateFlagsCmd,
							VolumeMounts:             []corev1.VolumeMount{outputVolumeMount},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
					},
					// Just "cat" the output file, to make it easily available for retrieval from the container logs
					// This bypasses the size limitations of TerminationMessagePath (4KB).
					Containers: []corev1.Container{
						{
							Name: getFlagsContainerName,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  new(int64(1000)),
								RunAsGroup: new(int64(1000)),
							},
							Image:                    image,
							ImagePullPolicy:          imagePullPolicy,
							Command:                  []string{"cat", getFlagsOutputPath},
							VolumeMounts:             []corev1.VolumeMount{outputVolumeMount},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
					},
				},
			},
		},
	}

	registry, _ := cluster.GetPrivateRegistry(nil)
	if registry != nil && len(registry.PullSecrets) > 0 {
		job.Spec.Template.Spec.ImagePullSecrets = registry.PullSecretsAsObjectReferences()
	}

	return l.jobClient.Create(job)
}

// getPodLogs retrieves the logs of a specific container in a pod
func (l *Lifecycle) getPodLogs(namespace, podName, containerName string) ([]byte, error) {
	req := l.k8sClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	stream, err := req.Stream(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error streaming logs for pod %s/%s container %s: %w", namespace, podName, containerName, err)
	}
	defer stream.Close()

	return io.ReadAll(stream)
}

// getJobStatus returns the status of a get-flags Job and the result payload if succeeded
func (l *Lifecycle) getJobStatus(jobName, expectedHash string) (status jobStatus, payload []byte, err error) {
	job, err := l.jobCache.Get(namespace.System, jobName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return jobStatusNotFound, nil, nil
		}
		return "", nil, err
	}

	// If the Job was created for an older configuration hash, it is stale
	if job.Annotations == nil || job.Annotations[driverHashAnnotation] != expectedHash {
		return jobStatusStale, nil, nil
	}

	// Check job conditions
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			// Job succeeded - read the pod's logs from the "print-flags" container
			pods, err := l.podCache.List(job.Namespace, labels.SelectorFromSet(labels.Set{batchv1.JobNameLabel: job.Name}))
			if err != nil {
				return "", nil, err
			}

			for _, pod := range pods {
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if containerStatus.Name == getFlagsContainerName && containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode == 0 {
						logs, err := l.getPodLogs(pod.Namespace, pod.Name, getFlagsContainerName)
						if err != nil {
							return "", nil, err
						}
						return jobStatusSucceeded, bytes.TrimSpace(logs), nil
					}
				}
			}
			return jobStatusSucceeded, nil, fmt.Errorf("job succeeded but %s container logs could not be retrieved", getFlagsContainerName)
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			// Job failed - read the failure message from container termination status
			pods, err := l.podCache.List(job.Namespace, labels.SelectorFromSet(labels.Set{batchv1.JobNameLabel: job.Name}))
			if err != nil {
				return jobStatusFailed, nil, err
			}

			for _, pod := range pods {
				allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
				for _, cs := range allStatuses {
					if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
						if msg := strings.TrimSpace(cs.State.Terminated.Message); msg != "" {
							return jobStatusFailed, []byte(msg), nil
						}
					}
				}
			}
			failMsg := cond.Message
			if failMsg == "" {
				failMsg = "get-create-flags job failed"
			}
			return jobStatusFailed, []byte(failMsg), nil
		}
	}

	return jobStatusRunning, nil, nil
}

// deleteJob deletes a get-flags Job with foreground propagation
func (l *Lifecycle) deleteJob(jobName string) error {
	propagation := metav1.DeletePropagationForeground
	return l.jobClient.Delete(namespace.System, jobName, &metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}
