// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

// Package kstatus contains functionality for computing the status
// of Kubernetes resources.
//
// The statuses defined in this package are:
//  * InProgress
//  * Current
//  * Failed
//  * Terminating
//  * Unknown
//
// Computing the status of a resources can be done by calling the
// Compute function in the status package.
//
//   import (
//     "sigs.k8s.io/cli-utils/pkg/kstatus/status"
//   )
//
//   res, err := status.Compute(resource)
//
// The package also defines a set of new conditions:
//  * InProgress
//  * Failed
// These conditions have been chosen to follow the
// "abnormal-true" pattern where conditions should be set to true
// for error/abnormal conditions and the absence of a condition means
// things are normal.
//
// The Augment function augments any unstructured resource with
// the standard conditions described above. The values of
// these conditions are decided based on other status information
// available in the resources.
//
//   import (
//     "sigs.k8s.io/cli-utils/pkg/kstatus/status
//   )
//
//   err := status.Augment(resource)
package status
