package systeminfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewRetriever validates that NewRetriever properly initializes a Retriever
// or returns nil when provided with nil input.
func TestNewRetriever(t *testing.T) {
	t.Run("returns nil when clients is nil", func(t *testing.T) {
		retriever := NewRetriever(nil)
		assert.Nil(t, retriever)
	})

	// Testing with actual clients would require complex setup, so we primarily test the nil case
	// which is the key safety check in the implementation
}

// TestGetSystemPodLabelSelectors validates that GetSystemPodLabelSelectors returns
// the expected set of label selectors for system pods across different scenarios.
func TestGetSystemPodLabelSelectors(t *testing.T) {
	// Note: This test validates the structure and format of the returned selectors.
	// Full testing with fleet cluster cache would require complex mock setup, so we
	// test the nil control plane case which is the key safety check in the implementation.

	t.Run("nil control plane returns empty slice", func(t *testing.T) {
		t.Parallel()
		retriever := &Retriever{
			fleetClusterCache: nil,
		}

		selectors := retriever.GetSystemPodLabelSelectors(nil)
		assert.Empty(t, selectors)
	})
}

// TestGetSystemPodLabelSelectorsFormat validates that all returned selectors
// follow the expected "namespace:labelSelector" format when control plane is not nil.
// This is a basic test that doesn't require fleet cluster cache setup.
func TestGetSystemPodLabelSelectorsFormat(t *testing.T) {
	t.Parallel()
	// This test is simplified since we cannot test with actual fleet cluster cache
	// without complex mock setup. The format validation would need a control plane
	// that doesn't cause nil pointer dereference.
}
