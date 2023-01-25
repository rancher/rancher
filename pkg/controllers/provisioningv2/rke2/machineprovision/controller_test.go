package machineprovision

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
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
