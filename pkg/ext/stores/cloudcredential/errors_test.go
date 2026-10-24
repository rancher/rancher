package cloudcredential

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestMapBackingError(t *testing.T) {
	t.Parallel()

	t.Run("redacts backing identity and preserves classification", func(t *testing.T) {
		t.Parallel()
		err := mapBackingError(apierrors.NewConflict(schema.GroupResource{Group: "", Resource: "secrets"}, "backing-secret", errors.New("details")), "credential")
		require.True(t, apierrors.IsConflict(err))
		assert.Contains(t, err.Error(), "cloudcredentials.ext.cattle.io")
		assert.NotContains(t, err.Error(), "backing-secret")
	})

	t.Run("preserves retry after", func(t *testing.T) {
		t.Parallel()
		err := mapBackingError(apierrors.NewTooManyRequests("slow down", 17), "credential")
		require.True(t, apierrors.IsTooManyRequests(err))
		status := err.(apierrors.APIStatus).Status()
		assert.Equal(t, int32(17), status.Details.RetryAfterSeconds)
		assert.NotContains(t, err.Error(), "Secret")
	})

	t.Run("preserves invalid causes", func(t *testing.T) {
		t.Parallel()
		backing := apierrors.NewInvalid(schema.GroupKind{Group: "", Kind: "Secret"}, "backing-secret", field.ErrorList{
			field.Invalid(field.NewPath("data", "key"), "value", "bad"),
		})
		err := mapBackingError(backing, "credential")
		require.True(t, apierrors.IsInvalid(err))
		assert.Equal(t, GVK.Kind, err.(apierrors.APIStatus).Status().Details.Kind)
	})

	t.Run("preserves deliberate API status through wrapping", func(t *testing.T) {
		t.Parallel()
		err := apierrors.NewBadRequest("bad selector")
		assert.True(t, apierrors.IsBadRequest(apiStatusOrInternalError(errors.Join(errors.New("context"), err))))
	})

	t.Run("converts unexpected errors to internal", func(t *testing.T) {
		t.Parallel()
		assert.True(t, apierrors.IsInternalError(apiStatusOrInternalError(errors.New("boom"))))
	})
}
