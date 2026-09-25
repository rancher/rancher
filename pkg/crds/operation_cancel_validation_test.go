package crds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

// operationCRDs are the three operation types which share OperationSpec, and so share the
// root-level rule guarding spec.cancel.
var operationCRDs = []string{
	"etcdsnapshotsaves.operation.cattle.io",
	"etcdsnapshotrestores.operation.cattle.io",
	"encryptionkeyrotations.operation.cattle.io",
	"certificaterotations.operation.cattle.io",
}

// TestOperationCRDsAreAcceptable compiles each operation CRD the way the API server does when it is
// installed. A CEL rule that does not compile is rejected at install time, which would leave
// Rancher unable to create the CRD at all, and nothing short of this actually exercises the
// expression — controller-gen copies it into the schema without looking at it.
func TestOperationCRDsAreAcceptable(t *testing.T) {
	t.Parallel()

	for _, name := range operationCRDs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			errs := apiextvalidation.ValidateCustomResourceDefinition(context.Background(), internalCRD(t, name))
			assert.Empty(t, errs, "the API server would reject this CRD")
		})
	}
}

// TestOperationCancelValidation covers the root-level rule on spec.cancel. It is written as a
// root-level rule rather than on the field because deciding whether a cancellation can still take
// effect needs status.phase, which a rule scoped to the field cannot see.
//
// The case that most needs pinning is the status update: the rule must not fire when the controller
// moves an operation with cancel already set into a terminal phase, which is exactly what happens
// on every cancellation. A rule written without the oldSelf clause would compile, behave correctly
// for a user setting the field, and deadlock every cancellation the controller tried to record.
func TestOperationCancelValidation(t *testing.T) {
	t.Parallel()

	op := func(cancel bool, status map[string]interface{}) map[string]interface{} {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"clusterRef": map[string]interface{}{"name": "test"},
				"cancel":     cancel,
			},
		}
		if status != nil {
			obj["status"] = status
		}
		return obj
	}
	phase := func(p string) map[string]interface{} {
		return map[string]interface{}{"phase": p}
	}

	cases := []struct {
		name     string
		old      map[string]interface{}
		updated  map[string]interface{}
		rejected bool
	}{
		{
			name:    "cancel set while pending",
			old:     op(false, phase("Pending")),
			updated: op(true, phase("Pending")),
		},
		{
			name:    "cancel set while in progress",
			old:     op(false, phase("InProgress")),
			updated: op(true, phase("InProgress")),
		},
		{
			name: "cancel set while paused",
			// Pausing defers a cancellation rather than defeating it, so the request is accepted and
			// acted on once the pause lifts.
			old:     op(false, map[string]interface{}{"phase": "InProgress"}),
			updated: op(true, map[string]interface{}{"phase": "InProgress"}),
		},
		{
			name:    "cancel set before the operation has any status",
			old:     op(false, nil),
			updated: op(true, nil),
		},
		{
			name:    "cancel set before the operation has a phase",
			old:     op(false, map[string]interface{}{}),
			updated: op(true, map[string]interface{}{}),
		},
		{
			name:     "cancel set once succeeded",
			old:      op(false, phase("Succeeded")),
			updated:  op(true, phase("Succeeded")),
			rejected: true,
		},
		{
			name:     "cancel set once failed",
			old:      op(false, phase("Failed")),
			updated:  op(true, phase("Failed")),
			rejected: true,
		},
		{
			name:     "cancel set once aborted",
			old:      op(false, phase("Aborted")),
			updated:  op(true, phase("Aborted")),
			rejected: true,
		},
		{
			name:     "cancel set once canceled",
			old:      op(false, phase("Canceled")),
			updated:  op(true, phase("Canceled")),
			rejected: true,
		},
		{
			// The status update every cancellation ends in: spec is unchanged, so the rule must not
			// fire, or the controller could never record the phase it just moved the operation to.
			name:    "controller records the terminal phase of a canceled operation",
			old:     op(true, phase("InProgress")),
			updated: op(true, phase("Canceled")),
		},
		{
			// Likewise for an operation which was canceled and then concluded some other way, and
			// for any later write (a finalizer edit, say) to an already-canceled operation.
			name:    "unrelated update to an operation already carrying cancel",
			old:     op(true, phase("Succeeded")),
			updated: op(true, phase("Succeeded")),
		},
	}

	for _, name := range operationCRDs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			validator := rootValidator(t, name)

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					errs, _ := validator.Validate(context.Background(), field.NewPath(""), nil,
						tc.updated, tc.old, celconfig.RuntimeCELCostBudget)

					if tc.rejected {
						assert.NotEmpty(t, errs, "the update should have been rejected")
						return
					}
					assert.Empty(t, errs, "the update should have been accepted")
				})
			}
		})
	}
}

// internalCRD loads a generated CRD and converts it to the internal type the validation and
// structural-schema packages work with. The status the API server would populate on create is
// filled in too, since ValidateCustomResourceDefinition validates the object as stored rather than
// as submitted, and a generated manifest carries no status at all.
func internalCRD(t *testing.T, name string) *apiextensions.CustomResourceDefinition {
	t.Helper()

	all, err := crdsFromDir(baseDir)
	require.NoError(t, err, "reading the embedded CRD yaml")

	v1CRD, found := all[name]
	require.True(t, found, "CRD %s not found in the embedded file system", name)

	internal := &apiextensions.CustomResourceDefinition{}
	require.NoError(t, apiextv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(v1CRD, internal, nil))

	internal.Status.AcceptedNames = internal.Spec.Names
	for _, version := range internal.Spec.Versions {
		if version.Storage {
			internal.Status.StoredVersions = append(internal.Status.StoredVersions, version.Name)
		}
	}

	return internal
}

// rootValidator builds the CEL validator for the root schema of the named CRD, which is where a
// rule that needs to read both spec and status has to live.
func rootValidator(t *testing.T, name string) *structuralcel.Validator {
	t.Helper()

	internal := internalCRD(t, name)

	// Converting to the internal type hoists a schema shared by every version up to the spec and
	// clears the per-version copies, so the single-version CRDs here carry it in spec.validation.
	props := schemaOf(internal.Spec.Validation)
	for _, version := range internal.Spec.Versions {
		if version.Storage && props == nil {
			props = schemaOf(version.Schema)
		}
	}
	require.NotNil(t, props, "no schema on %s", name)

	structural, err := structuralschema.NewStructural(props)
	require.NoError(t, err, "building the structural schema")

	validator := structuralcel.NewValidator(structural, true, celconfig.PerCallLimit)
	require.NotNil(t, validator, "the root schema declares no CEL rules")

	return validator
}

// schemaOf returns the root schema of a CustomResourceValidation, tolerating a nil holder.
func schemaOf(validation *apiextensions.CustomResourceValidation) *apiextensions.JSONSchemaProps {
	if validation == nil {
		return nil
	}
	return validation.OpenAPIV3Schema
}
