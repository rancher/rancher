/*
Copyright 2017 The Kubernetes Authors.

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
// should be set in the Machine.Status. The "Reason" field is meant for short,
// enum-style constants meant to be interpreted by machines. The "Message"
// field is meant to be read by humans.
type MachineError struct {
	Reason  MachineStatusError
	Message string
}

func (e *MachineError) Error() string {
	return e.Message
}

// Some error builders for ease of use. They set the appropriate "Reason"
// value, and all arguments are Printf-style varargs fed into Sprintf to
// construct the Message.

func InvalidMachineConfiguration(msg string, args ...interface{}) *MachineError {
	return &MachineError{
		Reason:  InvalidConfigurationMachineError,
		Message: fmt.Sprintf(msg, args...),
	}
}

func CreateMachine(msg string, args ...interface{}) *MachineError {
	return &MachineError{
		Reason:  CreateMachineError,
		Message: fmt.Sprintf(msg, args...),
	}
}

func UpdateMachine(msg string, args ...interface{}) *MachineError {
	return &MachineError{
		Reason:  UpdateMachineError,
		Message: fmt.Sprintf(msg, args...),
	}
}

func DeleteMachine(msg string, args ...interface{}) *MachineError {
	return &MachineError{
		Reason:  DeleteMachineError,
		Message: fmt.Sprintf(msg, args...),
	}
}
