package provisioningcluster

import (
	"errors"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestProvisioningClusterController_reconcileConditions(t *testing.T) {
	type dummyObject struct {
		Status struct {
			Conditions []genericcondition.GenericCondition
		}
	}
	type args struct {
		conditionA                       condition.Cond
		conditionB                       condition.Cond
		objectAConditionAMessage         string
		objectAConditionAReason          string
		objectAConditionAStatus          string
		objectBConditionAMessage         string
		objectBConditionAReason          string
		objectBConditionAStatus          string
		objectAConditionBMessage         string
		objectAConditionBReason          string
		objectAConditionBStatus          string
		objectBConditionBMessage         string
		objectBConditionBReason          string
		objectBConditionBStatus          string
		expectedObjectAConditionAMessage string
		expectedObjectAConditionAReason  string
		expectedObjectAConditionAStatus  string
		expectedObjectBConditionAMessage string
		expectedObjectBConditionAReason  string
		expectedObjectBConditionAStatus  string
		expectedObjectAConditionBMessage string
		expectedObjectAConditionBReason  string
		expectedObjectAConditionBStatus  string
		expectedObjectBConditionBMessage string
		expectedObjectBConditionBReason  string
		expectedObjectBConditionBStatus  string
		expectChange                     bool
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "basic copy",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "myMessage",
				objectAConditionAReason:          "myReason",
				objectAConditionAStatus:          "myStatus",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "",
				objectBConditionBReason:          "",
				objectBConditionBStatus:          "",
				expectedObjectAConditionAMessage: "myMessage",
				expectedObjectAConditionAReason:  "myReason",
				expectedObjectAConditionAStatus:  "myStatus",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "myMessage",
				expectedObjectBConditionBReason:  "myReason",
				expectedObjectBConditionBStatus:  "myStatus",
				expectChange:                     true,
			},
		},
		{
			name: "basic erase",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "",
				objectAConditionAReason:          "",
				objectAConditionAStatus:          "",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "",
				expectedObjectAConditionAReason:  "",
				expectedObjectAConditionAStatus:  "",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "",
				expectedObjectBConditionBReason:  "",
				expectedObjectBConditionBStatus:  "",
				expectChange:                     true,
			},
		},
		{
			name: "basic overwrite",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "",
				objectBConditionAReason:          "",
				objectBConditionAStatus:          "",
				objectAConditionBMessage:         "",
				objectAConditionBReason:          "",
				objectAConditionBStatus:          "",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "",
				expectedObjectBConditionAReason:  "",
				expectedObjectBConditionAStatus:  "",
				expectedObjectAConditionBMessage: "",
				expectedObjectAConditionBReason:  "",
				expectedObjectAConditionBStatus:  "",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     true,
			},
		},
		{
			name: "copy around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "myMessage",
				objectAConditionAReason:          "myReason",
				objectAConditionAStatus:          "myStatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "",
				objectBConditionBReason:          "",
				objectBConditionBStatus:          "",
				expectedObjectAConditionAMessage: "myMessage",
				expectedObjectAConditionAReason:  "myReason",
				expectedObjectAConditionAStatus:  "myStatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "myMessage",
				expectedObjectBConditionBReason:  "myReason",
				expectedObjectBConditionBStatus:  "myStatus",
				expectChange:                     true,
			},
		},
		{
			name: "erase around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "",
				objectAConditionAReason:          "",
				objectAConditionAStatus:          "",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "",
				expectedObjectAConditionAReason:  "",
				expectedObjectAConditionAStatus:  "",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "",
				expectedObjectBConditionBReason:  "",
				expectedObjectBConditionBStatus:  "",
				expectChange:                     true,
			},
		},
		{
			name: "overwrite around other conditions",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "myMessage",
				objectBConditionBReason:          "myReason",
				objectBConditionBStatus:          "myStatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     true,
			},
		},
		{
			name: "no changes",
			args: args{
				conditionA:                       condition.Cond("conditionA"),
				conditionB:                       condition.Cond("conditionB"),
				objectAConditionAMessage:         "wow",
				objectAConditionAReason:          "wowreason",
				objectAConditionAStatus:          "wowstatus",
				objectBConditionAMessage:         "donttouch",
				objectBConditionAReason:          "thiscondition",
				objectBConditionAStatus:          "becauseifyoudoitsbroken",
				objectAConditionBMessage:         "orthisone",
				objectAConditionBReason:          "andthisone",
				objectAConditionBStatus:          "alsothisone",
				objectBConditionBMessage:         "wow",
				objectBConditionBReason:          "wowreason",
				objectBConditionBStatus:          "wowstatus",
				expectedObjectAConditionAMessage: "wow",
				expectedObjectAConditionAReason:  "wowreason",
				expectedObjectAConditionAStatus:  "wowstatus",
				expectedObjectBConditionAMessage: "donttouch",
				expectedObjectBConditionAReason:  "thiscondition",
				expectedObjectBConditionAStatus:  "becauseifyoudoitsbroken",
				expectedObjectAConditionBMessage: "orthisone",
				expectedObjectAConditionBReason:  "andthisone",
				expectedObjectAConditionBStatus:  "alsothisone",
				expectedObjectBConditionBMessage: "wow",
				expectedObjectBConditionBReason:  "wowreason",
				expectedObjectBConditionBStatus:  "wowstatus",
				expectChange:                     false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests copying object A condition A to object B condition B.
			a := assert.New(t)
			var objectA = dummyObject{}
			var objectB = dummyObject{}

			if tt.args.objectAConditionAMessage != "" {
				tt.args.conditionA.Message(&objectA, tt.args.objectAConditionAMessage)
			}
			if tt.args.objectAConditionAReason != "" {
				tt.args.conditionA.Reason(&objectA, tt.args.objectAConditionAReason)
			}
			if tt.args.objectAConditionAStatus != "" {
				tt.args.conditionA.SetStatus(&objectA, tt.args.objectAConditionAStatus)
			}
			if tt.args.objectAConditionBMessage != "" {
				tt.args.conditionB.Message(&objectA, tt.args.objectAConditionBMessage)
			}
			if tt.args.objectAConditionBReason != "" {
				tt.args.conditionB.Reason(&objectA, tt.args.objectAConditionBReason)
			}
			if tt.args.objectAConditionBStatus != "" {
				tt.args.conditionB.SetStatus(&objectA, tt.args.objectAConditionBStatus)
			}
			if tt.args.objectBConditionAMessage != "" {
				tt.args.conditionA.Message(&objectB, tt.args.objectBConditionAMessage)
			}
			if tt.args.objectBConditionAReason != "" {
				tt.args.conditionA.Reason(&objectB, tt.args.objectBConditionAReason)
			}
			if tt.args.objectBConditionAStatus != "" {
				tt.args.conditionA.SetStatus(&objectB, tt.args.objectBConditionAStatus)
			}
			if tt.args.objectBConditionBMessage != "" {
				tt.args.conditionB.Message(&objectB, tt.args.objectBConditionBMessage)
			}
			if tt.args.objectBConditionBReason != "" {
				tt.args.conditionB.Reason(&objectB, tt.args.objectBConditionBReason)
			}
			if tt.args.objectBConditionBStatus != "" {
				tt.args.conditionB.SetStatus(&objectB, tt.args.objectBConditionBStatus)
			}

			a.Equal(tt.args.objectAConditionAMessage, tt.args.conditionA.GetMessage(&objectA))
			a.Equal(tt.args.objectAConditionAReason, tt.args.conditionA.GetReason(&objectA))
			a.Equal(tt.args.objectAConditionAStatus, tt.args.conditionA.GetStatus(&objectA))
			a.Equal(tt.args.objectAConditionBMessage, tt.args.conditionB.GetMessage(&objectA))
			a.Equal(tt.args.objectAConditionBReason, tt.args.conditionB.GetReason(&objectA))
			a.Equal(tt.args.objectAConditionBStatus, tt.args.conditionB.GetStatus(&objectA))

			a.Equal(tt.args.objectBConditionAMessage, tt.args.conditionA.GetMessage(&objectB))
			a.Equal(tt.args.objectBConditionAReason, tt.args.conditionA.GetReason(&objectB))
			a.Equal(tt.args.objectBConditionAStatus, tt.args.conditionA.GetStatus(&objectB))
			a.Equal(tt.args.objectBConditionBMessage, tt.args.conditionB.GetMessage(&objectB))
			a.Equal(tt.args.objectBConditionBReason, tt.args.conditionB.GetReason(&objectB))
			a.Equal(tt.args.objectBConditionBStatus, tt.args.conditionB.GetStatus(&objectB))

			a.Equal(tt.args.expectChange, reconcileCondition(&objectB, tt.args.conditionB, &objectA, tt.args.conditionA))

			a.Equal(tt.args.expectedObjectAConditionAMessage, tt.args.conditionA.GetMessage(&objectA))
			a.Equal(tt.args.expectedObjectAConditionAReason, tt.args.conditionA.GetReason(&objectA))
			a.Equal(tt.args.expectedObjectAConditionAStatus, tt.args.conditionA.GetStatus(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBMessage, tt.args.conditionB.GetMessage(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBReason, tt.args.conditionB.GetReason(&objectA))
			a.Equal(tt.args.expectedObjectAConditionBStatus, tt.args.conditionB.GetStatus(&objectA))

			a.Equal(tt.args.expectedObjectBConditionAMessage, tt.args.conditionA.GetMessage(&objectB))
			a.Equal(tt.args.expectedObjectBConditionAReason, tt.args.conditionA.GetReason(&objectB))
			a.Equal(tt.args.expectedObjectBConditionAStatus, tt.args.conditionA.GetStatus(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBMessage, tt.args.conditionB.GetMessage(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBReason, tt.args.conditionB.GetReason(&objectB))
			a.Equal(tt.args.expectedObjectBConditionBStatus, tt.args.conditionB.GetStatus(&objectB))
		})
	}

}

func TestGeneratingHandler(t *testing.T) {
	tests := []struct {
		name           string
		obj            *provv1.Cluster
		expected       []runtime.Object
		expectedStatus provv1.ClusterStatus
		expectedErr    error
	}{
		{
			name:           "no rkeconfig",
			obj:            &provv1.Cluster{},
			expected:       nil,
			expectedStatus: provv1.ClusterStatus{},
			expectedErr:    nil,
		},
		{
			name: "no cluster name",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
			},
			expected:       nil,
			expectedStatus: provv1.ClusterStatus{},
			expectedErr:    nil,
		},
		{
			name: "deleting",
			obj: &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &[]metav1.Time{metav1.Now()}[0],
				},
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: nil,
		},
		{
			name: "k8s version",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: errors.New("kubernetesVersion not set on /"),
		},
		{
			name: "no finalizers",
			obj: &provv1.Cluster{
				Spec: provv1.ClusterSpec{
					KubernetesVersion: "test-version",
					RKEConfig:         &provv1.RKEConfig{},
				},
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expected: nil,
			expectedStatus: provv1.ClusterStatus{
				ClusterName: "test-cluster",
			},
			expectedErr: generic.ErrSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{}
			var status provv1.ClusterStatus
			if tt.obj != nil {
				status = tt.obj.Status
			}
			objs, generatedStatus, err := h.OnRancherClusterChange(tt.obj, status)
			assert.Equal(t, tt.expected, objs)
			assert.Equal(t, tt.expectedStatus, generatedStatus)
			if tt.expectedErr != nil {
				assert.NotNil(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
