package summary

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SummarizedObject struct {
	metav1.PartialObjectMetadata
	Summary
}

type SummarizedObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SummarizedObject `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func Summarized(u runtime.Object) *SummarizedObject {
	if s, ok := u.(*SummarizedObject); ok {
		return s
	}

	s := &SummarizedObject{
		Summary: Summarize(u),
	}
	s.APIVersion, s.Kind = u.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	meta, err := meta.Accessor(u)
	if err == nil {
		s.Name = meta.GetName()
		s.Namespace = meta.GetNamespace()
		s.Generation = meta.GetGeneration()
		s.UID = meta.GetUID()
		s.ResourceVersion = meta.GetResourceVersion()
		s.CreationTimestamp = meta.GetCreationTimestamp()
		s.DeletionTimestamp = meta.GetDeletionTimestamp()
		s.Labels = meta.GetLabels()
		s.Annotations = meta.GetAnnotations()
	}

	return s
}

func (in *SummarizedObjectList) DeepCopyInto(out *SummarizedObjectList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SummarizedObject, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

func (in *SummarizedObjectList) DeepCopy() *SummarizedObjectList {
	if in == nil {
		return nil
	}
	out := new(SummarizedObjectList)
	in.DeepCopyInto(out)
	return out
}

func (in *SummarizedObjectList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *SummarizedObject) DeepCopyInto(out *SummarizedObject) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Summary = *in.Summary.DeepCopy()
	return
}

func (in *SummarizedObject) DeepCopy() *SummarizedObject {
	if in == nil {
		return nil
	}
	out := new(SummarizedObject)
	in.DeepCopyInto(out)
	return out
}

func (in *SummarizedObject) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
