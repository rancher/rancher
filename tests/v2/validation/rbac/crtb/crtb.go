package crtb

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

const (
	cattleSystemNamespace                           = "cattle-system"
	customClusterRoleName                           = "custom-cluster-owner"
	deploymentName                                  = "rancher"
	deploymentEnvVarName                            = "CATTLE_RESYNC_DEFAULT"
	TrueConditionStatus      metav1.ConditionStatus = "True"
	FalseConditionStatus     metav1.ConditionStatus = "False"
	CompletedSummary                                = "Completed"
	SubjectExists                                   = "SubjectExists"
	LabelsReconciled                                = "LabelsReconciled"
	BindingExists                                   = "BindingExists"
	CRTBLabelsUpdated                               = "CRTBLabelsUpdated"
	ClusterRolesExist                               = "ClusterRolesExist"
	ClusterRoleBindingsExists                       = "ClusterRoleBindingsExists"
	ServiceAccountImpersonatorExists                = "ServiceAccountImpersonatorExists"
)

func verifyClusterRoleTemplateBindingStatusField(crtb *v3.ClusterRoleTemplateBinding) error {
	status := crtb.Status

	_, err := time.Parse(time.RFC3339, status.LastUpdateTime)
	if err != nil {
		return fmt.Errorf("lastUpdateTime is invalid: %w", err)
	}

	requiredLocalConditions := []string{
		SubjectExists,
		LabelsReconciled,
		BindingExists,
	}
	for _, condition := range status.LocalConditions {
		for _, reqType := range requiredLocalConditions {
			if condition.Type == reqType {
				if condition.Status != TrueConditionStatus {
					return fmt.Errorf("%s condition is not True. Actual status: %s", reqType, condition.Status)
				}

				if condition.LastTransitionTime.IsZero() {
					return fmt.Errorf("%s lastTransitionTime is not set or invalid", reqType)
				}

				if condition.Message != "" {
					return fmt.Errorf("%s message should be empty. Actual message: %s", reqType, condition.Message)
				}

				if condition.Reason != condition.Type {
					return fmt.Errorf("Expected: %s, Actual: %s", condition.Type, condition.Reason)
				}
			}
		}
	}
	
	if status.ObservedGenerationLocal != 2 {
		return fmt.Errorf("observedGenerationLocal is not 2, found: %d", status.ObservedGenerationLocal)
	}

	if status.Summary != CompletedSummary || status.SummaryLocal != CompletedSummary {
		return fmt.Errorf("summary or summaryLocal is not 'Completed'")
	}

	requiredRemoteConditions := []string{
		CRTBLabelsUpdated,
		ClusterRolesExist,
		ClusterRoleBindingsExists,
		ServiceAccountImpersonatorExists,
	}
	for _, condition := range status.RemoteConditions {
		for _, reqType := range requiredRemoteConditions {
			if condition.Type == reqType {
				if condition.Status != TrueConditionStatus {
					return fmt.Errorf("%s condition is not True. Actual status: %s", reqType, condition.Status)
				}

				if condition.LastTransitionTime.IsZero() {
					return fmt.Errorf("%s lastTransitionTime is not set or invalid", reqType)
				}

				if condition.Message != "" {
					return fmt.Errorf("%s message should be empty. Actual message: %s", reqType, condition.Message)
				}

				if condition.Reason != condition.Type {
					return fmt.Errorf("Expected: %s, Actual: %s", condition.Type, condition.Reason)
				}
			}
		}
	}

	if status.ObservedGenerationRemote != 2 {
		return fmt.Errorf("observedGenerationRemote is not 2, found: %d", status.ObservedGenerationRemote)
	}

	if status.SummaryRemote != CompletedSummary {
		return fmt.Errorf("summaryRemote is not 'Completed'")
	}
	
	return nil
}