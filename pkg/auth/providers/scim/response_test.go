package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorError(t *testing.T) {
	tests := []struct {
		name   string
		err    Error
		expect string
	}{
		{
			name:   "status only",
			err:    Error{Status: 404},
			expect: "status: 404",
		},
		{
			name:   "status with detail",
			err:    Error{Status: 400, Detail: "invalid request"},
			expect: "status: 400, detail: invalid request",
		},
		{
			name:   "status with empty detail",
			err:    Error{Status: 500, Detail: ""},
			expect: "status: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.err.Error())
		})
	}
}

func TestErrorMarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		err    Error
		expect map[string]any
	}{
		{
			name: "basic error",
			err: Error{
				Schemas: []string{errorSchemaID},
				Status:  400,
				Detail:  "Bad request",
			},
			expect: map[string]any{
				"schemas": []any{errorSchemaID},
				"status":  "400",
				"detail":  "Bad request",
			},
		},
		{
			name: "error with scimType",
			err: Error{
				Schemas:  []string{errorSchemaID},
				Status:   409,
				Detail:   "Resource already exists",
				ScimType: "uniqueness",
			},
			expect: map[string]any{
				"schemas":  []any{errorSchemaID},
				"status":   "409",
				"detail":   "Resource already exists",
				"scimType": "uniqueness",
			},
		},
		{
			name: "error without scimType excludes field",
			err: Error{
				Schemas: []string{errorSchemaID},
				Status:  404,
				Detail:  "Not found",
			},
			expect: map[string]any{
				"schemas": []any{errorSchemaID},
				"status":  "404",
				"detail":  "Not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.err)
			require.NoError(t, err)

			var result map[string]any
			require.NoError(t, json.Unmarshal(data, &result))

			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		detail   string
		scimType []string
		expect   *Error
	}{
		{
			name:   "basic error",
			status: 400,
			detail: "Bad request",
			expect: &Error{
				Schemas: []string{errorSchemaID},
				Status:  400,
				Detail:  "Bad request",
			},
		},
		{
			name:     "error with scimType",
			status:   409,
			detail:   "Resource already exists",
			scimType: []string{"uniqueness"},
			expect: &Error{
				Schemas:  []string{errorSchemaID},
				Status:   409,
				Detail:   "Resource already exists",
				ScimType: "uniqueness",
			},
		},
		{
			name:     "error with multiple scimTypes uses first",
			status:   400,
			detail:   "Invalid value",
			scimType: []string{"invalidValue", "mutability"},
			expect: &Error{
				Schemas:  []string{errorSchemaID},
				Status:   400,
				Detail:   "Invalid value",
				ScimType: "invalidValue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewError(tt.status, tt.detail, tt.scimType...)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError()

	assert.Equal(t, []string{errorSchemaID}, err.Schemas)
	assert.Equal(t, http.StatusInternalServerError, err.Status)
	assert.Equal(t, "Internal Server Error", err.Detail)
	assert.Empty(t, err.ScimType)
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name         string
		err          *Error
		expectStatus int
		expectBody   map[string]any
	}{
		{
			name:         "writes bad request error",
			err:          NewError(http.StatusBadRequest, "Invalid input"),
			expectStatus: http.StatusBadRequest,
			expectBody: map[string]any{
				"schemas": []any{errorSchemaID},
				"status":  "400",
				"detail":  "Invalid input",
			},
		},
		{
			name:         "writes not found error",
			err:          NewError(http.StatusNotFound, "Resource not found"),
			expectStatus: http.StatusNotFound,
			expectBody: map[string]any{
				"schemas": []any{errorSchemaID},
				"status":  "404",
				"detail":  "Resource not found",
			},
		},
		{
			name:         "writes error with scimType",
			err:          NewError(http.StatusConflict, "Duplicate", "uniqueness"),
			expectStatus: http.StatusConflict,
			expectBody: map[string]any{
				"schemas":  []any{errorSchemaID},
				"status":   "409",
				"detail":   "Duplicate",
				"scimType": "uniqueness",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)

			assert.Equal(t, tt.expectStatus, w.Result().StatusCode)
			assert.Equal(t, "application/scim+json", w.Result().Header.Get("Content-Type"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.Equal(t, tt.expectBody, body)
		})
	}
}

func TestWriteResponse(t *testing.T) {
	t.Run("no payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeResponse(w, noPayload, http.StatusNoContent)

		assert.Equal(t, http.StatusNoContent, w.Result().StatusCode)
		assert.Equal(t, "", w.Result().Header.Get("Content-Type"))
		assert.Equal(t, 0, w.Body.Len())
	})

	t.Run("with payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := map[string]string{"key": "value"}
		writeResponse(w, payload, http.StatusOK)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Equal(t, "application/scim+json", w.Result().Header.Get("Content-Type"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
		assert.Equal(t, payload, body)
	})

	t.Run("with payload default status", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := map[string]string{"name": "test"}
		writeResponse(w, payload)

		// Default status is 200 when WriteHeader is not called
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Equal(t, "application/scim+json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("with list response", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := listResponse{
			Schemas:      []string{listSchemaID},
			TotalResults: 10,
			ItemsPerPage: 5,
			StartIndex:   1,
			Resources:    []any{"item1", "item2"},
		}
		writeResponse(w, payload, http.StatusOK)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Equal(t, "application/scim+json", w.Result().Header.Get("Content-Type"))

		var body listResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
		assert.Equal(t, payload, body)
	})

	t.Run("nil payload no content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeResponse(w, nil, http.StatusOK)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Equal(t, "", w.Result().Header.Get("Content-Type"))
		assert.Equal(t, 0, w.Body.Len())
	})

	t.Run("with created status", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := map[string]string{"id": "123"}
		writeResponse(w, payload, http.StatusCreated)

		assert.Equal(t, http.StatusCreated, w.Result().StatusCode)
		assert.Equal(t, "application/scim+json", w.Result().Header.Get("Content-Type"))
	})
}
