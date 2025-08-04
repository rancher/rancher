package machineprovision

import (
	"errors"
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestGetJobFailureTime(t *testing.T) {

	fakeTime := time.Date(2023, time.August, 13, 10, 0, 0, 0, time.UTC)

	testCases := []struct {
		name          string
		job           *batchv1.Job
		expectedTime  time.Time
		expectedError error
	}{
		{
			name: "job with valid fail time",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fakeTime}},
					},
				},
			},
			expectedTime:  fakeTime,
			expectedError: nil,
		},
		{
			name: "job with no valid fail time",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fakeTime}},
					},
				},
			},
			expectedTime:  fakeTime,
			expectedError: errors.New("job failure condition not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getJobFailureTime(tc.job)
			if tc.expectedError == nil {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedTime, got)
			} else {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			}
		})
	}

}

func TestGetInfraDeleteAction(t *testing.T) {

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	testCases := []struct {
		name                 string
		job                  *batchv1.Job
		deleteOnFailureAfter string
		expectedAction       string
	}{
		{
			name: "job that must be enqueued",
			job: &batchv1.Job{

				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fiveMinutesAgo}},
					},
				},
			},
			deleteOnFailureAfter: "6m",
			expectedAction:       "enqueue",
		},
		{
			name: "job that must be deleted",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fiveMinutesAgo}},
					},
				},
			},
			deleteOnFailureAfter: "4m",
			expectedAction:       "delete",
		},
		{
			name: "job that must do default behavior",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fiveMinutesAgo}},
					},
				},
			},
			deleteOnFailureAfter: "0",
			expectedAction:       "delete",
		},
		{
			name: "job that must do nothing",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Time{Time: fiveMinutesAgo}},
					},
				},
			},
			deleteOnFailureAfter: "-1s",
			expectedAction:       "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings.DeleteMachineJobOnFailureAfter.Set(tc.deleteOnFailureAfter)
			gotAction, _, gotError := getInfraDeletionAction(tc.job)
			assert.Equal(t, tc.expectedAction, gotAction)
			assert.Nil(t, gotError)
		})
	}

}
