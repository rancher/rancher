package v3

import (
	reflect "reflect"

	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
//
// Deprecated: deepcopy registration will go away when static deepcopy is fully implemented.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*BasicAuth).DeepCopyInto(out.(*BasicAuth))
			return nil
		}, InType: reflect.TypeOf(&BasicAuth{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*BasicAuthList).DeepCopyInto(out.(*BasicAuthList))
			return nil
		}, InType: reflect.TypeOf(&BasicAuthList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Certificate).DeepCopyInto(out.(*Certificate))
			return nil
		}, InType: reflect.TypeOf(&Certificate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CertificateList).DeepCopyInto(out.(*CertificateList))
			return nil
		}, InType: reflect.TypeOf(&CertificateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeployConfig).DeepCopyInto(out.(*DeployConfig))
			return nil
		}, InType: reflect.TypeOf(&DeployConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeployStrategy).DeepCopyInto(out.(*DeployStrategy))
			return nil
		}, InType: reflect.TypeOf(&DeployStrategy{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeploymentGlobalConfig).DeepCopyInto(out.(*DeploymentGlobalConfig))
			return nil
		}, InType: reflect.TypeOf(&DeploymentGlobalConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeploymentJobConfig).DeepCopyInto(out.(*DeploymentJobConfig))
			return nil
		}, InType: reflect.TypeOf(&DeploymentJobConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeploymentOrderedConfig).DeepCopyInto(out.(*DeploymentOrderedConfig))
			return nil
		}, InType: reflect.TypeOf(&DeploymentOrderedConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DeploymentParallelConfig).DeepCopyInto(out.(*DeploymentParallelConfig))
			return nil
		}, InType: reflect.TypeOf(&DeploymentParallelConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DockerCredential).DeepCopyInto(out.(*DockerCredential))
			return nil
		}, InType: reflect.TypeOf(&DockerCredential{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DockerCredentialList).DeepCopyInto(out.(*DockerCredentialList))
			return nil
		}, InType: reflect.TypeOf(&DockerCredentialList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Link).DeepCopyInto(out.(*Link))
			return nil
		}, InType: reflect.TypeOf(&Link{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedBasicAuth).DeepCopyInto(out.(*NamespacedBasicAuth))
			return nil
		}, InType: reflect.TypeOf(&NamespacedBasicAuth{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedBasicAuthList).DeepCopyInto(out.(*NamespacedBasicAuthList))
			return nil
		}, InType: reflect.TypeOf(&NamespacedBasicAuthList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedCertificate).DeepCopyInto(out.(*NamespacedCertificate))
			return nil
		}, InType: reflect.TypeOf(&NamespacedCertificate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedCertificateList).DeepCopyInto(out.(*NamespacedCertificateList))
			return nil
		}, InType: reflect.TypeOf(&NamespacedCertificateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedDockerCredential).DeepCopyInto(out.(*NamespacedDockerCredential))
			return nil
		}, InType: reflect.TypeOf(&NamespacedDockerCredential{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedDockerCredentialList).DeepCopyInto(out.(*NamespacedDockerCredentialList))
			return nil
		}, InType: reflect.TypeOf(&NamespacedDockerCredentialList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedSSHAuth).DeepCopyInto(out.(*NamespacedSSHAuth))
			return nil
		}, InType: reflect.TypeOf(&NamespacedSSHAuth{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedSSHAuthList).DeepCopyInto(out.(*NamespacedSSHAuthList))
			return nil
		}, InType: reflect.TypeOf(&NamespacedSSHAuthList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedServiceAccountToken).DeepCopyInto(out.(*NamespacedServiceAccountToken))
			return nil
		}, InType: reflect.TypeOf(&NamespacedServiceAccountToken{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NamespacedServiceAccountTokenList).DeepCopyInto(out.(*NamespacedServiceAccountTokenList))
			return nil
		}, InType: reflect.TypeOf(&NamespacedServiceAccountTokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RegistryCredential).DeepCopyInto(out.(*RegistryCredential))
			return nil
		}, InType: reflect.TypeOf(&RegistryCredential{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SSHAuth).DeepCopyInto(out.(*SSHAuth))
			return nil
		}, InType: reflect.TypeOf(&SSHAuth{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SSHAuthList).DeepCopyInto(out.(*SSHAuthList))
			return nil
		}, InType: reflect.TypeOf(&SSHAuthList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ServiceAccountToken).DeepCopyInto(out.(*ServiceAccountToken))
			return nil
		}, InType: reflect.TypeOf(&ServiceAccountToken{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ServiceAccountTokenList).DeepCopyInto(out.(*ServiceAccountTokenList))
			return nil
		}, InType: reflect.TypeOf(&ServiceAccountTokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Workload).DeepCopyInto(out.(*Workload))
			return nil
		}, InType: reflect.TypeOf(&Workload{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*WorkloadList).DeepCopyInto(out.(*WorkloadList))
			return nil
		}, InType: reflect.TypeOf(&WorkloadList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*WorkloadSpec).DeepCopyInto(out.(*WorkloadSpec))
			return nil
		}, InType: reflect.TypeOf(&WorkloadSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*WorkloadStatus).DeepCopyInto(out.(*WorkloadStatus))
			return nil
		}, InType: reflect.TypeOf(&WorkloadStatus{})},
	)
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BasicAuth) DeepCopyInto(out *BasicAuth) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BasicAuth.
func (in *BasicAuth) DeepCopy() *BasicAuth {
	if in == nil {
		return nil
	}
	out := new(BasicAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BasicAuth) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BasicAuthList) DeepCopyInto(out *BasicAuthList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BasicAuth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BasicAuthList.
func (in *BasicAuthList) DeepCopy() *BasicAuthList {
	if in == nil {
		return nil
	}
	out := new(BasicAuthList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BasicAuthList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Certificate) DeepCopyInto(out *Certificate) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.SubjectAlternativeNames != nil {
		in, out := &in.SubjectAlternativeNames, &out.SubjectAlternativeNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Certificate.
func (in *Certificate) DeepCopy() *Certificate {
	if in == nil {
		return nil
	}
	out := new(Certificate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Certificate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertificateList) DeepCopyInto(out *CertificateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Certificate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertificateList.
func (in *CertificateList) DeepCopy() *CertificateList {
	if in == nil {
		return nil
	}
	out := new(CertificateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CertificateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeployConfig) DeepCopyInto(out *DeployConfig) {
	*out = *in
	if in.DeploymentStrategy != nil {
		in, out := &in.DeploymentStrategy, &out.DeploymentStrategy
		if *in == nil {
			*out = nil
		} else {
			*out = new(DeployStrategy)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeployConfig.
func (in *DeployConfig) DeepCopy() *DeployConfig {
	if in == nil {
		return nil
	}
	out := new(DeployConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeployStrategy) DeepCopyInto(out *DeployStrategy) {
	*out = *in
	if in.ParallelConfig != nil {
		in, out := &in.ParallelConfig, &out.ParallelConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(DeploymentParallelConfig)
			**out = **in
		}
	}
	if in.JobConfig != nil {
		in, out := &in.JobConfig, &out.JobConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(DeploymentJobConfig)
			**out = **in
		}
	}
	if in.OrderedConfig != nil {
		in, out := &in.OrderedConfig, &out.OrderedConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(DeploymentOrderedConfig)
			**out = **in
		}
	}
	if in.GlobalConfig != nil {
		in, out := &in.GlobalConfig, &out.GlobalConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(DeploymentGlobalConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeployStrategy.
func (in *DeployStrategy) DeepCopy() *DeployStrategy {
	if in == nil {
		return nil
	}
	out := new(DeployStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentGlobalConfig) DeepCopyInto(out *DeploymentGlobalConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentGlobalConfig.
func (in *DeploymentGlobalConfig) DeepCopy() *DeploymentGlobalConfig {
	if in == nil {
		return nil
	}
	out := new(DeploymentGlobalConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentJobConfig) DeepCopyInto(out *DeploymentJobConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentJobConfig.
func (in *DeploymentJobConfig) DeepCopy() *DeploymentJobConfig {
	if in == nil {
		return nil
	}
	out := new(DeploymentJobConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentOrderedConfig) DeepCopyInto(out *DeploymentOrderedConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentOrderedConfig.
func (in *DeploymentOrderedConfig) DeepCopy() *DeploymentOrderedConfig {
	if in == nil {
		return nil
	}
	out := new(DeploymentOrderedConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentParallelConfig) DeepCopyInto(out *DeploymentParallelConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentParallelConfig.
func (in *DeploymentParallelConfig) DeepCopy() *DeploymentParallelConfig {
	if in == nil {
		return nil
	}
	out := new(DeploymentParallelConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockerCredential) DeepCopyInto(out *DockerCredential) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Registries != nil {
		in, out := &in.Registries, &out.Registries
		*out = make(map[string]RegistryCredential, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockerCredential.
func (in *DockerCredential) DeepCopy() *DockerCredential {
	if in == nil {
		return nil
	}
	out := new(DockerCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockerCredential) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockerCredentialList) DeepCopyInto(out *DockerCredentialList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DockerCredential, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockerCredentialList.
func (in *DockerCredentialList) DeepCopy() *DockerCredentialList {
	if in == nil {
		return nil
	}
	out := new(DockerCredentialList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DockerCredentialList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Link) DeepCopyInto(out *Link) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Link.
func (in *Link) DeepCopy() *Link {
	if in == nil {
		return nil
	}
	out := new(Link)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedBasicAuth) DeepCopyInto(out *NamespacedBasicAuth) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedBasicAuth.
func (in *NamespacedBasicAuth) DeepCopy() *NamespacedBasicAuth {
	if in == nil {
		return nil
	}
	out := new(NamespacedBasicAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedBasicAuth) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedBasicAuthList) DeepCopyInto(out *NamespacedBasicAuthList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespacedBasicAuth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedBasicAuthList.
func (in *NamespacedBasicAuthList) DeepCopy() *NamespacedBasicAuthList {
	if in == nil {
		return nil
	}
	out := new(NamespacedBasicAuthList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedBasicAuthList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedCertificate) DeepCopyInto(out *NamespacedCertificate) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.SubjectAlternativeNames != nil {
		in, out := &in.SubjectAlternativeNames, &out.SubjectAlternativeNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedCertificate.
func (in *NamespacedCertificate) DeepCopy() *NamespacedCertificate {
	if in == nil {
		return nil
	}
	out := new(NamespacedCertificate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedCertificate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedCertificateList) DeepCopyInto(out *NamespacedCertificateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespacedCertificate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedCertificateList.
func (in *NamespacedCertificateList) DeepCopy() *NamespacedCertificateList {
	if in == nil {
		return nil
	}
	out := new(NamespacedCertificateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedCertificateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedDockerCredential) DeepCopyInto(out *NamespacedDockerCredential) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Registries != nil {
		in, out := &in.Registries, &out.Registries
		*out = make(map[string]RegistryCredential, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedDockerCredential.
func (in *NamespacedDockerCredential) DeepCopy() *NamespacedDockerCredential {
	if in == nil {
		return nil
	}
	out := new(NamespacedDockerCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedDockerCredential) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedDockerCredentialList) DeepCopyInto(out *NamespacedDockerCredentialList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespacedDockerCredential, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedDockerCredentialList.
func (in *NamespacedDockerCredentialList) DeepCopy() *NamespacedDockerCredentialList {
	if in == nil {
		return nil
	}
	out := new(NamespacedDockerCredentialList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedDockerCredentialList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedSSHAuth) DeepCopyInto(out *NamespacedSSHAuth) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedSSHAuth.
func (in *NamespacedSSHAuth) DeepCopy() *NamespacedSSHAuth {
	if in == nil {
		return nil
	}
	out := new(NamespacedSSHAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedSSHAuth) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedSSHAuthList) DeepCopyInto(out *NamespacedSSHAuthList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespacedSSHAuth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedSSHAuthList.
func (in *NamespacedSSHAuthList) DeepCopy() *NamespacedSSHAuthList {
	if in == nil {
		return nil
	}
	out := new(NamespacedSSHAuthList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedSSHAuthList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedServiceAccountToken) DeepCopyInto(out *NamespacedServiceAccountToken) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedServiceAccountToken.
func (in *NamespacedServiceAccountToken) DeepCopy() *NamespacedServiceAccountToken {
	if in == nil {
		return nil
	}
	out := new(NamespacedServiceAccountToken)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedServiceAccountToken) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedServiceAccountTokenList) DeepCopyInto(out *NamespacedServiceAccountTokenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NamespacedServiceAccountToken, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedServiceAccountTokenList.
func (in *NamespacedServiceAccountTokenList) DeepCopy() *NamespacedServiceAccountTokenList {
	if in == nil {
		return nil
	}
	out := new(NamespacedServiceAccountTokenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NamespacedServiceAccountTokenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RegistryCredential) DeepCopyInto(out *RegistryCredential) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RegistryCredential.
func (in *RegistryCredential) DeepCopy() *RegistryCredential {
	if in == nil {
		return nil
	}
	out := new(RegistryCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SSHAuth) DeepCopyInto(out *SSHAuth) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SSHAuth.
func (in *SSHAuth) DeepCopy() *SSHAuth {
	if in == nil {
		return nil
	}
	out := new(SSHAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SSHAuth) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SSHAuthList) DeepCopyInto(out *SSHAuthList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SSHAuth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SSHAuthList.
func (in *SSHAuthList) DeepCopy() *SSHAuthList {
	if in == nil {
		return nil
	}
	out := new(SSHAuthList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SSHAuthList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceAccountToken) DeepCopyInto(out *ServiceAccountToken) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceAccountToken.
func (in *ServiceAccountToken) DeepCopy() *ServiceAccountToken {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountToken)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceAccountToken) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceAccountTokenList) DeepCopyInto(out *ServiceAccountTokenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceAccountToken, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceAccountTokenList.
func (in *ServiceAccountTokenList) DeepCopy() *ServiceAccountTokenList {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountTokenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceAccountTokenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Workload) DeepCopyInto(out *Workload) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		if *in == nil {
			*out = nil
		} else {
			*out = new(WorkloadStatus)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Workload.
func (in *Workload) DeepCopy() *Workload {
	if in == nil {
		return nil
	}
	out := new(Workload)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Workload) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadList) DeepCopyInto(out *WorkloadList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Workload, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadList.
func (in *WorkloadList) DeepCopy() *WorkloadList {
	if in == nil {
		return nil
	}
	out := new(WorkloadList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkloadList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadSpec) DeepCopyInto(out *WorkloadSpec) {
	*out = *in
	in.DeployConfig.DeepCopyInto(&out.DeployConfig)
	in.Template.DeepCopyInto(&out.Template)
	if in.ServiceLinks != nil {
		in, out := &in.ServiceLinks, &out.ServiceLinks
		*out = make([]Link, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadSpec.
func (in *WorkloadSpec) DeepCopy() *WorkloadSpec {
	if in == nil {
		return nil
	}
	out := new(WorkloadSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadStatus) DeepCopyInto(out *WorkloadStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadStatus.
func (in *WorkloadStatus) DeepCopy() *WorkloadStatus {
	if in == nil {
		return nil
	}
	out := new(WorkloadStatus)
	in.DeepCopyInto(out)
	return out
}
