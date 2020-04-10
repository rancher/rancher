/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

// MachineStatusErrorPtr converts a MachineStatusError to a pointer.
func MachineStatusErrorPtr(v MachineStatusError) *MachineStatusError {
	return &v
}

// MachinePoolStatusErrorPtr converts a MachinePoolStatusError to a pointer.
func MachinePoolStatusErrorPtr(v MachinePoolStatusFailure) *MachinePoolStatusFailure {
	return &v
}

// ClusterStatusErrorPtr converts a MachineStatusError to a pointer.
func ClusterStatusErrorPtr(v ClusterStatusError) *ClusterStatusError {
	return &v
}
