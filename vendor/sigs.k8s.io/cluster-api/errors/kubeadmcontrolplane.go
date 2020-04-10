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

// KubeadmControlPlaneError is a more descriptive kind of error that represents an error condition that
// should be set in the KubeadmControlPlane.Status. The "Reason" field is meant for short,
// enum-style constants meant to be interpreted by control planes. The "Message"
// field is meant to be read by humans.
type KubeadmControlPlaneError struct {
	Reason  KubeadmControlPlaneStatusError
	Message string
}

func (e *KubeadmControlPlaneError) Error() string {
	return e.Message
}
