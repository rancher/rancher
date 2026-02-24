package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
)

// listResponse defines a SCIM list response.
type listResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Resources    []any    `json:"Resources"`
}

// Error defines a SCIM error response.
type Error struct {
	Schemas  []string `json:"schemas"`
	Status   int      `json:"status"`
	Detail   string   `json:"detail"`
	ScimType string   `json:"scimType,omitempty"`
}

// Error implements the [error] interface.
func (e Error) Error() string {
	s := fmt.Sprintf("status: %d", e.Status)
	if e.Detail != "" {
		s += fmt.Sprintf(", detail: %s", e.Detail)
	}
	return s
}

// MarshalJSON customizes the JSON marshaling for Error.
func (e Error) MarshalJSON() ([]byte, error) {
	t := map[string]any{
		"schemas": e.Schemas,
		"status":  strconv.Itoa(e.Status),
		"detail":  e.Detail,
	}
	if e.ScimType != "" {
		t["scimType"] = e.ScimType
	}

	return json.Marshal(t)
}

func (e *Error) UnmarshalJSON(data []byte) error {
	t := struct {
		Schemas  []string `json:"schemas"`
		Status   string   `json:"status"`
		Detail   string   `json:"detail"`
		ScimType string   `json:"scimType,omitempty"`
	}{}
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	e.Schemas = t.Schemas
	e.Detail = t.Detail
	e.ScimType = t.ScimType

	if t.Status != "" {
		status, err := strconv.Atoi(t.Status)
		if err != nil {
			return fmt.Errorf("invalid status value: %s", t.Status)
		}
		e.Status = status
	}

	return nil
}

// NewError creates a new SCIM error.
func NewError(status int, detail string, scimType ...string) *Error {
	err := &Error{
		Schemas: []string{errorSchemaID},
		Status:  status,
		Detail:  detail,
	}

	if s := first(scimType); s != "" {
		err.ScimType = s
	}

	return err
}

// NewInternalError creates a new SCIM internal server error.
func NewInternalError() *Error {
	return NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

// writeError writes a SCIM error response.
func writeError(w http.ResponseWriter, err *Error) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(err.Status)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		logrus.Errorf("scim::writeError: failed to encode response: %s", err)
	}
}

// writeResponse writes a SCIM response.
func writeResponse(w http.ResponseWriter, payload any, status ...int) {
	if payload != nil {
		w.Header().Set("Content-Type", "application/scim+json")
	}

	if s := first(status); s > 0 {
		w.WriteHeader(s)
	}

	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logrus.Errorf("scim::writeResponse: failed to encode response: %s", err)
	}
}

// noPayload represents an empty response payload.
var noPayload any
