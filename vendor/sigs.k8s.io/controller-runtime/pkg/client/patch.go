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

package client

import (
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
)

var (
	// Apply uses server-side apply to patch the given object.
	Apply = applyPatch{}

	// Merge uses the raw object as a merge patch, without modifications.
	// Use MergeFrom if you wish to compute a diff instead.
	Merge = mergePatch{}
)

type patch struct {
	patchType types.PatchType
	data      []byte
}

// Type implements Patch.
func (s *patch) Type() types.PatchType {
	return s.patchType
}

// Data implements Patch.
func (s *patch) Data(obj runtime.Object) ([]byte, error) {
	return s.data, nil
}

// RawPatch constructs a new Patch with the given PatchType and data.
func RawPatch(patchType types.PatchType, data []byte) Patch {
	return &patch{patchType, data}
}

// ConstantPatch constructs a new Patch with the given PatchType and data.
//
// Deprecated: use RawPatch instead
func ConstantPatch(patchType types.PatchType, data []byte) Patch {
	return RawPatch(patchType, data)
}

type mergeFromPatch struct {
	from runtime.Object
}

// Type implements patch.
func (s *mergeFromPatch) Type() types.PatchType {
	return types.MergePatchType
}

// Data implements Patch.
func (s *mergeFromPatch) Data(obj runtime.Object) ([]byte, error) {
	originalJSON, err := json.Marshal(s.from)
	if err != nil {
		return nil, err
	}

	modifiedJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
}

// MergeFrom creates a Patch that patches using the merge-patch strategy with the given object as base.
func MergeFrom(obj runtime.Object) Patch {
	return &mergeFromPatch{obj}
}

// mergePatch uses a raw merge strategy to patch the object.
type mergePatch struct{}

// Type implements Patch.
func (p mergePatch) Type() types.PatchType {
	return types.MergePatchType
}

// Data implements Patch.
func (p mergePatch) Data(obj runtime.Object) ([]byte, error) {
	// NB(directxman12): we might technically want to be using an actual encoder
	// here (in case some more performant encoder is introduced) but this is
	// correct and sufficient for our uses (it's what the JSON serializer in
	// client-go does, more-or-less).
	return json.Marshal(obj)
}

// applyPatch uses server-side apply to patch the object.
type applyPatch struct{}

// Type implements Patch.
func (p applyPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

// Data implements Patch.
func (p applyPatch) Data(obj runtime.Object) ([]byte, error) {
	// NB(directxman12): we might technically want to be using an actual encoder
	// here (in case some more performant encoder is introduced) but this is
	// correct and sufficient for our uses (it's what the JSON serializer in
	// client-go does, more-or-less).
	return json.Marshal(obj)
}
