// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// checkGenericProperties looks at the properties that are available on
// all or most of the Kubernetes resources. If a decision can be made based
// on this information, there is no need to look at the resource-specidic
// rules.
// This also checks for the presence of the conditions defined in this package.
// If any of these are set on the resource, a decision is made solely based
// on this and none of the resource specific rules will be used. The goal here
// is that if controllers, built-in or custom, use these conditions, we can easily
// find status of resources.
func checkGenericProperties(u *unstructured.Unstructured) (*Result, error) {
	obj := u.UnstructuredContent()

	// Check if the resource is scheduled for deletion
	deletionTimestamp, found, err := unstructured.NestedString(obj, "metadata", "deletionTimestamp")
	if err != nil {
		return nil, errors.Wrap(err, "looking up metadata.deletionTimestamp from resource")
	}
	if found && deletionTimestamp != "" {
		return &Result{
			Status:     TerminatingStatus,
			Message:    "Resource scheduled for deletion",
			Conditions: []Condition{},
		}, nil
	}

	res, err := checkGeneration(u)
	if res != nil || err != nil {
		return res, err
	}

	// Check if the resource has any of the standard conditions. If so, we just use them
	// and no need to look at anything else.
	objWithConditions, err := GetObjectWithConditions(obj)
	if err != nil {
		return nil, err
	}

	for _, cond := range objWithConditions.Status.Conditions {
		if cond.Type == string(ConditionReconciling) && cond.Status == corev1.ConditionTrue {
			return newInProgressStatus(cond.Reason, cond.Message), nil
		}
		if cond.Type == string(ConditionStalled) && cond.Status == corev1.ConditionTrue {
			return &Result{
				Status:  FailedStatus,
				Message: cond.Message,
				Conditions: []Condition{
					{
						Type:    ConditionStalled,
						Status:  corev1.ConditionTrue,
						Reason:  cond.Reason,
						Message: cond.Message,
					},
				},
			}, nil
		}
	}

	return nil, nil
}

func checkGeneration(u *unstructured.Unstructured) (*Result, error) {
	// ensure that the meta generation is observed
	generation, found, err := unstructured.NestedInt64(u.Object, "metadata", "generation")
	if err != nil {
		return nil, errors.Wrap(err, "looking up metadata.generation from resource")
	}
	if !found {
		return nil, nil
	}
	observedGeneration, found, err := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	if err != nil {
		return nil, errors.Wrap(err, "looking up status.observedGeneration from resource")
	}
	if found {
		// Resource does not have this field, so we can't do this check.
		// TODO(mortent): Verify behavior of not set vs does not exist.
		if observedGeneration != generation {
			message := fmt.Sprintf("%s generation is %d, but latest observed generation is %d", u.GetKind(), generation, observedGeneration)
			return &Result{
				Status:     InProgressStatus,
				Message:    message,
				Conditions: []Condition{newReconcilingCondition("LatestGenerationNotObserved", message)},
			}, nil
		}
	}
	return nil, nil
}
