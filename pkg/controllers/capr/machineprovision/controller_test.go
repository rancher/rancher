package machineprovision

import (
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/apply/fake"
	"github.com/rancher/wrangler/v3/pkg/data"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/batch/v1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

func MustStatus(status rkev1.RKEMachineStatus, err error) rkev1.RKEMachineStatus {
	if err != nil {
		panic(err)
	}
	return status
}

func TestReconcileStatus(t *testing.T) {
	h := handler{}

	tests := []struct {
		name     string
		expected map[string]interface{}
		input    map[string]interface{}
		state    rkev1.RKEMachineStatus
	}{
		{
			name: "create complete",
			expected: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "CreateJob",
						},
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "Ready",
						},
					},
				},
			},
			input: map[string]interface{}{},
			state: MustStatus(h.getMachineStatus(&batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   "Complete",
							Status: "True",
						},
					},
				},
			})),
		},
		{
			name: "create in progress",
			expected: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"message": "creating server [test-namespace/infraMachineName] of kind (infraMachineKind) for machine capiMachineName in infrastructure provider",
							"reason":  "",
							"status":  "False",
							"type":    "Ready",
						},
					},
				},
			},
			input: map[string]interface{}{},
			state: MustStatus(h.getMachineStatus(&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								InfraMachineName: "infraMachineName",
								InfraMachineKind: "infraMachineKind",
								CapiMachineName:  "capiMachineName",
							},
						},
					},
				},
			})),
		},
		{
			name: "create complete delete complete",
			expected: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "CreateJob",
						},
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "Ready",
						},
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "DeleteJob",
						},
					},
				},
			},
			input: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "CreateJob",
						},
						map[string]interface{}{
							"message": "",
							"reason":  "",
							"status":  "True",
							"type":    "Ready",
						},
					},
				},
			},
			state: MustStatus(h.getMachineStatus(&batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								InfraJobRemove: "true",
							},
						},
					},
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   "Complete",
							Status: "True",
						},
					},
				},
			})),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconcileStatus(tt.input, tt.state)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestConstructFilesSecret(t *testing.T) {
	testCases := []struct {
		name               string
		inputAliasedFields string
		config             map[string]interface{}
		expectedSecret     *corev1.Secret
	}{
		{
			name:               "no fileToFieldAliases annotation",
			inputAliasedFields: "",
			config: map[string]interface{}{
				"sshPort":  "22",
				"userdata": "/path/to/machine/files/userdata",
			},
			expectedSecret: nil,
		},
		{
			name:               "known driver with fileToFieldAliases annotation",
			inputAliasedFields: "userdata:userdata",
			config: map[string]interface{}{
				"sshPort":  "22",
				"userdata": "/path/to/machine/files/userdata",
			},
			expectedSecret: &corev1.Secret{
				Data: map[string][]byte{
					"userdata": []byte("/path/to/machine/files/userdata\n"),
				},
			},
		},
		{
			name:               "custom driver with fileToFieldAliases annotation",
			inputAliasedFields: "foo:bar",
			config: map[string]interface{}{
				"foo":     "randomValue",
				"sshPort": "22",
			},
			expectedSecret: &corev1.Secret{
				Data: map[string][]byte{
					"bar": []byte("randomValue\n"),
				},
			},
		},
		{
			name:               "empty config content",
			inputAliasedFields: "foo:bar",
			config: map[string]interface{}{
				"foo": "",
			},
			expectedSecret: &corev1.Secret{
				Data: map[string][]byte{},
			},
		},
		{
			name:               "sshKey field config changes",
			inputAliasedFields: "sshKeyContents:sshKeyPath",
			config: map[string]interface{}{
				"sshKeyContents": "/path/to/machine/files/sshContent",
			},
			expectedSecret: &corev1.Secret{
				Data: map[string][]byte{
					"id_rsa": []byte("/path/to/machine/files/sshContent\n"),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secret := constructFilesSecret(tc.inputAliasedFields, tc.config)
			assert.Equal(t, tc.expectedSecret, secret)
		})
	}
}

func TestInfraMachineDeletionEnqueueTime(t *testing.T) {

	ctrl := gomock.NewController(t)
	now := time.Now()

	testCases := []struct {
		name         string
		h            handler
		setting      string
		infra        *infraObject
		expectedTime time.Duration
	}{
		{
			name:         "setting is positive, should delete",
			h:            handler{jobs: newJob(ctrl, now.Add(-10*time.Minute))},
			setting:      "5m",
			infra:        newInfra("jobName"),
			expectedTime: 0,
		},
		{
			name:         "settings is zero, should return zero",
			h:            handler{jobs: newJob(ctrl, now.Add(-10*time.Minute))},
			setting:      "0s",
			infra:        newInfra("jobName"),
			expectedTime: 0,
		},
		{
			name:         "setting is positive, should enqueue",
			h:            handler{jobs: newJob(ctrl, now.Add(-1*time.Minute))},
			setting:      "5m",
			infra:        newInfra("jobName"),
			expectedTime: 4 * time.Minute,
		},
		{
			name:         "setting is negative, should delete",
			h:            handler{jobs: newJob(ctrl, now.Add(-10*time.Minute))},
			setting:      "-5m",
			infra:        newInfra("jobName"),
			expectedTime: 0,
		},
		{
			name:         "infra has no job name, should delete",
			h:            handler{jobs: newJob(ctrl, now.Add(-10*time.Minute))},
			setting:      "5m",
			infra:        newInfra(""),
			expectedTime: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settingInDuration, _ := time.ParseDuration(tc.setting)
			gotTime, err := tc.h.infraMachineDeletionEnqueueingTime(tc.infra, now, settingInDuration)
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedTime, gotTime)
		})
	}
}

func TestOnChange(t *testing.T) {
	ctrl := gomock.NewController(t)

	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
	machineCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(newCapiMachine("name", "namespace"), nil).AnyTimes()

	machineClient := ctrlfake.NewMockControllerInterface[*capi.Machine, *capi.MachineList](ctrl)

	clusterCache := ctrlfake.NewMockCacheInterface[*capi.Cluster](ctrl)
	clusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(newCluster("name", "namespace"), nil).AnyTimes()

	nodeDriverCache := ctrlfake.NewMockNonNamespacedCacheInterface[*apimgmtv3.NodeDriver](ctrl)
	nodeDriverCache.EXPECT().Get(gomock.Any()).Return(&apimgmtv3.NodeDriver{}, nil).AnyTimes()

	rancherClusterCache := ctrlfake.NewMockCacheInterface[*rancherv1.Cluster](ctrl)
	rancherClusterCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&rancherv1.Cluster{}, nil).AnyTimes()

	secrets := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secrets.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&corev1.Secret{}, nil).AnyTimes()

	apply := fake.FakeApply{}
	now := metav1.Now()
	h := &handler{
		machineCache:        machineCache,
		machineClient:       machineClient,
		capiClusterCache:    clusterCache,
		nodeDriverCache:     nodeDriverCache,
		rancherClusterCache: rancherClusterCache,
		secrets:             secrets,
		apply:               &apply,
	}

	testCases := []struct {
		name        string
		setting     string
		action      string
		jobCache    v1.JobCache
		withFailure bool
	}{
		{
			name:        "setting is positive with failure machine, should delete",
			setting:     "5m",
			action:      "delete",
			jobCache:    newJob(ctrl, now.Add(-10*time.Minute)),
			withFailure: true,
		},
		{
			name:        "setting is zero, should delete",
			setting:     "0m",
			action:      "delete",
			jobCache:    newJob(ctrl, now.Add(-10*time.Minute)),
			withFailure: true,
		},
		{
			name:        "object didn't failed, do not delete",
			setting:     "0m",
			action:      "nothing",
			jobCache:    newJob(ctrl, now.Add(-10*time.Minute)),
			withFailure: false,
		},
		{
			name:        "object failed now and setting is smaller, should enqueue",
			setting:     "10m",
			action:      "enqueue",
			jobCache:    newJob(ctrl, now.Add(-5*time.Minute)),
			withFailure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings.DeleteMachineOnFailureAfter.Set(tc.setting)

			dynamicControllerFake := dynamicControllerFake{}
			h.dynamic = &dynamicControllerFake
			h.jobs = tc.jobCache

			if tc.action == "delete" {
				machineClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			}
			if tc.action != "delete" {
				machineClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			}
			_, err := h.OnChange(newTestInfraMachine(tc.withFailure)) // call for the tests

			if tc.action == "enqueue" {
				assert.Equal(t, 1, dynamicControllerFake.EnqueueAfterCalled)
			}
			assert.NoError(t, err)
		})
	}
}

func newCapiMachine(name, namespace string) *capi.Machine {
	dataSecretName := "a"
	return &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				capi.ClusterNameLabel: name,
			},
		},
		Spec: capi.MachineSpec{
			Bootstrap: capi.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: "v1",
				},
				DataSecretName: &dataSecretName,
			},
		},
	}
}

func newCluster(name, namespace string) *capi.Cluster {
	return &capi.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: capi.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				capi.ClusterNameLabel: name,
			},
		},
		Spec: capi.ClusterSpec{},
		Status: capi.ClusterStatus{
			InfrastructureReady: true,
		},
	}
}
func newJob(ctrl *gomock.Controller, t time.Time) *ctrlfake.MockCacheInterface[*batchv1.Job] {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job",
			Namespace: "namespace",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
					LastTransitionTime: metav1.Time{
						Time: t,
					},
				},
			},
		},
	}

	mockJobCache := ctrlfake.NewMockCacheInterface[*batchv1.Job](ctrl)
	mockJobCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&job, nil).AnyTimes()
	return mockJobCache
}

func newInfra(jobName string) *infraObject {
	return &infraObject{
		meta: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "namespace",
				},
			},
		},
		data: data.Object{
			"status": map[string]interface{}{
				"jobName": jobName,
			},
		},
	}
}

// newTestInfraMachine creates an object that will be translated to a infraMachine on the OnChange function
func newTestInfraMachine(withFailure bool) *unstructured.Unstructured {
	objectData := map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta1",
		"kind":       "InfraMachine",
		"metadata": map[string]interface{}{
			"namespace": "namespace",
			"name":      "name",
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": capi.GroupVersion.String(),
					"kind":       "Machine",
					"controller": true,
				},
			},
			"labels": map[string]interface{}{
				"rke.cattle.io/capi-machine-name": "name",
			},
		},
	}

	if withFailure {
		objectData["status"] = map[string]interface{}{
			"failureReason": string(capierrors.CreateMachineError),
			"jobName":       "job",
		}
	}

	return &unstructured.Unstructured{
		Object: objectData,
	}
}
