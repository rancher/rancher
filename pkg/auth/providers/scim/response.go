package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
)

type ListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Resources    []any    `json:"Resources"`
}

type Error struct {
	Schemas  []string `json:"schemas"`
	Status   int      `json:"status"`
	Detail   string   `json:"detail"`
	ScimType string   `json:"scimType,omitempty"`
}

func (e Error) Error() string {
	s := fmt.Sprintf("status: %d", e.Status)
	if e.Detail != "" {
		s += fmt.Sprintf(", detail: %s", e.Detail)
	}
	return s
}

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

func NewError(status int, detail string, scimType ...string) *Error {
	err := &Error{
		Schemas: []string{ErrorSchemaID},
		Status:  status,
		Detail:  detail,
	}

	if len(scimType) > 0 {
		err.ScimType = scimType[0]
	}

	return err
}

func NewInternalError() *Error {
	return NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

func writeError(w http.ResponseWriter, err *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		logrus.Errorf("scim::writeError: failed to encode response: %s", err)
	}
}

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

var noPayload any = nil
