package cred

import (
	"net/http"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCreateStore captures the data map passed to Create so tests can inspect it.
type fakeCreateStore struct {
	types.Store
	capturedData map[string]any
	err          error
}

func (f *fakeCreateStore) Create(_ *types.APIContext, _ *types.Schema, data map[string]any) (map[string]any, error) {
	// Store a copy so mutations after the call don't affect the assertion.
	f.capturedData = make(map[string]any, len(data))
	for k, v := range data {
		f.capturedData[k] = v
	}
	return data, f.err
}

func TestStore_Create_StampsCreatorLabel_NoExistingLabels(t *testing.T) {
	inner := &fakeCreateStore{}
	s := &Store{Store: inner}

	req, err := http.NewRequest(http.MethodPost, "/v3/cloudcredentials", nil)
	require.NoError(t, err)
	req.Header.Set("Impersonate-User", "user-abc123")

	apiCtx := &types.APIContext{Request: req}
	data := map[string]any{"genericConfig": map[string]any{"apiKey": "secret"}}

	_, err = s.Create(apiCtx, nil, data)
	require.NoError(t, err)

	require.NotNil(t, inner.capturedData)
	labels, ok := inner.capturedData["labels"].(map[string]interface{})
	require.True(t, ok, "labels should be a map[string]interface{}")
	assert.Equal(t, "user-abc123", labels["cattle.io/creator"])
}

func TestStore_Create_StampsCreatorLabel_MergesWithExistingLabels(t *testing.T) {
	inner := &fakeCreateStore{}
	s := &Store{Store: inner}

	req, err := http.NewRequest(http.MethodPost, "/v3/cloudcredentials", nil)
	require.NoError(t, err)
	req.Header.Set("Impersonate-User", "user-xyz")

	apiCtx := &types.APIContext{Request: req}
	data := map[string]any{
		"genericConfig": map[string]any{"token": "tok"},
		"labels":        map[string]interface{}{"existing-label": "existing-value"},
	}

	_, err = s.Create(apiCtx, nil, data)
	require.NoError(t, err)

	labels, ok := inner.capturedData["labels"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "user-xyz", labels["cattle.io/creator"])
	assert.Equal(t, "existing-value", labels["existing-label"], "pre-existing labels must be preserved")
}

func TestStore_Create_NilRequest_NoLabelStamped(t *testing.T) {
	inner := &fakeCreateStore{}
	s := &Store{Store: inner}

	// apiContext.Request is nil — no header to read from, so no label should be written.
	apiCtx := &types.APIContext{Request: nil}
	data := map[string]any{"genericConfig": map[string]any{"token": "tok"}}

	_, err := s.Create(apiCtx, nil, data)
	require.NoError(t, err)

	_, hasLabels := inner.capturedData["labels"]
	assert.False(t, hasLabels, "no labels key should be added when Request is nil")
}

func TestStore_Create_EmptyUserID_StillSetsLabel(t *testing.T) {
	inner := &fakeCreateStore{}
	s := &Store{Store: inner}

	req, err := http.NewRequest(http.MethodPost, "/v3/cloudcredentials", nil)
	require.NoError(t, err)
	// No Impersonate-User header set → Header.Get returns "".

	apiCtx := &types.APIContext{Request: req}
	data := map[string]any{"genericConfig": map[string]any{"token": "tok"}}

	_, err = s.Create(apiCtx, nil, data)
	require.NoError(t, err)

	labels, ok := inner.capturedData["labels"].(map[string]interface{})
	require.True(t, ok, "label key should still be created even when user ID is empty")
	assert.Equal(t, "", labels["cattle.io/creator"])
}
