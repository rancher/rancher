//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2024 Rancher Labs, Inc.

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

// Code generated by main. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditLogPolicy) DeepCopyInto(out *AuditLogPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditLogPolicy.
func (in *AuditLogPolicy) DeepCopy() *AuditLogPolicy {
	if in == nil {
		return nil
	}
	out := new(AuditLogPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuditLogPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditLogPolicyList) DeepCopyInto(out *AuditLogPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AuditLogPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditLogPolicyList.
func (in *AuditLogPolicyList) DeepCopy() *AuditLogPolicyList {
	if in == nil {
		return nil
	}
	out := new(AuditLogPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuditLogPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditLogPolicySpec) DeepCopyInto(out *AuditLogPolicySpec) {
	*out = *in
	if in.Filters != nil {
		in, out := &in.Filters, &out.Filters
		*out = make([]Filter, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalRedactions != nil {
		in, out := &in.AdditionalRedactions, &out.AdditionalRedactions
		*out = make([]Redaction, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.Verbosity = in.Verbosity
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditLogPolicySpec.
func (in *AuditLogPolicySpec) DeepCopy() *AuditLogPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AuditLogPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditLogPolicyStatus) DeepCopyInto(out *AuditLogPolicyStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditLogPolicyStatus.
func (in *AuditLogPolicyStatus) DeepCopy() *AuditLogPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(AuditLogPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Filter) DeepCopyInto(out *Filter) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Filter.
func (in *Filter) DeepCopy() *Filter {
	if in == nil {
		return nil
	}
	out := new(Filter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogVerbosity) DeepCopyInto(out *LogVerbosity) {
	*out = *in
	out.Request = in.Request
	out.Response = in.Response
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogVerbosity.
func (in *LogVerbosity) DeepCopy() *LogVerbosity {
	if in == nil {
		return nil
	}
	out := new(LogVerbosity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Redaction) DeepCopyInto(out *Redaction) {
	*out = *in
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Paths != nil {
		in, out := &in.Paths, &out.Paths
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Keys != nil {
		in, out := &in.Keys, &out.Keys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Redaction.
func (in *Redaction) DeepCopy() *Redaction {
	if in == nil {
		return nil
	}
	out := new(Redaction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Verbosity) DeepCopyInto(out *Verbosity) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Verbosity.
func (in *Verbosity) DeepCopy() *Verbosity {
	if in == nil {
		return nil
	}
	out := new(Verbosity)
	in.DeepCopyInto(out)
	return out
}
