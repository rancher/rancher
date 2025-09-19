package machineprovision

import (
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/data"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func getJobCache(ctrl *gomock.Controller, t time.Time) *ctrlfake.MockCacheInterface[*batchv1.Job] {
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
						Time: t, // 10 minutes ago
					},
				},
			},
		},
	}

	mockJobCache := ctrlfake.NewMockCacheInterface[*batchv1.Job](ctrl)
	mockJobCache.EXPECT().Get("namespace", "job").Return(&job, nil).AnyTimes()
	return mockJobCache
}

func getInfraObject(namespace, jobName string) *infraObject {
	return &infraObject{
		meta: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": namespace,
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

func TestInfraMachineDeletionEnqueueTime(t *testing.T) {

	ctrl := gomock.NewController(t)

	testCases := []struct {
		name         string
		h            handler
		setting      string
		infra        *infraObject
		expectedTime time.Duration
	}{
		{
			name:         "setting is positive, should delete",
			h:            handler{jobs: getJobCache(ctrl, time.Now().Add(-10*time.Minute))},
			setting:      "5m",
			infra:        getInfraObject("namespace", "job"),
			expectedTime: 0,
		},
		{
			name:         "settings is zero, should return zero",
			h:            handler{jobs: getJobCache(ctrl, time.Now().Add(-10*time.Minute))},
			setting:      "0s",
			infra:        getInfraObject("namespace", "job"),
			expectedTime: 0,
		},
		{
			name:         "setting is positive, should enqueue",
			h:            handler{jobs: getJobCache(ctrl, time.Now().Add(-1*time.Minute))},
			setting:      "5m",
			infra:        getInfraObject("namespace", "job"),
			expectedTime: 4 * time.Minute,
		},
		{
			name:         "setting is negative, should delete",
			h:            handler{jobs: getJobCache(ctrl, time.Now().Add(-10*time.Minute))},
			setting:      "-5m",
			infra:        getInfraObject("namespace", "job"),
			expectedTime: 0,
		},
		{
			name:         "infra has no job name, should delete",
			h:            handler{jobs: getJobCache(ctrl, time.Now().Add(-10*time.Minute))},
			setting:      "-5m",
			infra:        getInfraObject("namespace", ""),
			expectedTime: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settingInDuration, _ := time.ParseDuration(tc.setting)
			gotTime, err := tc.h.infraMachineDeletionEnqueueingTime(tc.infra, settingInDuration)
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedTime.Round(time.Minute), gotTime.Round(time.Minute)) // it is not > exactly < equal, this is why use a round function, eg: 3m59s != 4m0s
		})
	}
}
