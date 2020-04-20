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

import (
	"fmt"
)

// A more descriptive kind of error that represents an error condition that
// should be set in the Cluster.Status. The "Reason" field is meant for short,
// enum-style constants meant to be interpreted by clusters. The "Message"
// field is meant to be read by humans.
type ClusterError struct {
	Reason  ClusterStatusError
	Message string
}

func (e *ClusterError) Error() string {
	return e.Message
}

// Some error builders for ease of use. They set the appropriate "Reason"
// value, and all arguments are Printf-style varargs fed into Sprintf to
// construct the Message.

func InvalidClusterConfiguration(format string, args ...interface{}) *ClusterError {
	return &ClusterError{
		Reason:  InvalidConfigurationClusterError,
		Message: fmt.Sprintf(format, args...),
	}
}

func CreateCluster(format string, args ...interface{}) *ClusterError {
	return &ClusterError{
		Reason:  CreateClusterError,
		Message: fmt.Sprintf(format, args...),
	}
}

func DeleteCluster(format string, args ...interface{}) *ClusterError {
	return &ClusterError{
		Reason:  DeleteClusterError,
		Message: fmt.Sprintf(format, args...),
	}
}
