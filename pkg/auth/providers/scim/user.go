package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Bool represents a boolean value that can be unmarshaled from JSON strings or boolean literals.
// Okta uses boolean values, whereas Azure uses strings "true"/"false" for primary email flag.
type Bool bool

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (b *Bool) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", `"true"`:
		*b = true
	case "false", `"false"`:
		*b = false
	default:
		return fmt.Errorf("invalid boolean value: %s", data)
	}
	return nil
}

// SCIMUser represents a SCIM user.
type SCIMUser struct {
	Active     bool   `json:"active"`
	ID         string `json:"id"`
	ExternalID string `json:"externalId"`
	Name       struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	DisplayName string `json:"displayName"`
	UserName    string `json:"userName"`
	Emails      []struct {
		Value   string `json:"value"`
		Primary Bool   `json:"primary"`
	} `json:"emails"`
	Schemas []string `json:"schemas"`
	Meta    struct {
		Created      string `json:"created"`
		ResourceType string `json:"resourceType"`
	}
}

// ListUsers returns a list of users.
// It supports filtering by userName using the "eq" operator.
// Pagination is supported via startIndex (1-based) and count query parameters.
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::ListUsers: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	// Parse pagination parameters.
	pagination, err := ParsePaginationParams(r)
	if err != nil {
		writeError(w, NewError(http.StatusBadRequest, err.Error()))
		return
	}

	// Parse filter.
	var nameFilter string
	if value := r.URL.Query().Get("filter"); value != "" {
		// For simplicity, only support filtering by userName eq "<value>"
		parts := strings.SplitN(value, " ", 3)
		if len(parts) != 3 || parts[0] != "userName" || parts[1] != "eq" {
			writeError(w, NewError(http.StatusBadRequest, "Unsupported filter"))
			return
		}
		nameFilter, err = url.QueryUnescape(strings.Trim(parts[2], `"`))
		if err != nil {
			writeError(w, NewError(http.StatusBadRequest, "Invalid filter value"))
			return
		}
	}

	logrus.Tracef("scim::ListUsers: userName=%s, startIndex=%d, count=%d", nameFilter, pagination.StartIndex, pagination.Count)

	list, err := s.userCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("scim::ListUsers: failed to list users: %s", err)
		writeError(w, NewInternalError())
		return
	}

	// Sort users by Name for deterministic ordering across pagination requests.
	// Without sorting, the cache order is undefined and pagination would be inconsistent.
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	// Collect all matching resources (needed to compute totalResults).
	var allResources []any
	for _, user := range list {
		if user.IsSystem() {
			continue
		}

		attr, err := s.userAttributeCache.Get(user.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				continue
			}
			logrus.Errorf("scim::ListUsers: failed to get user attributes for %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}

		values := attr.ExtraByProvider[provider]["principalid"]
		if len(values) == 0 {
			continue
		}

		userName := first(attr.ExtraByProvider[provider]["username"])
		externalID := first(attr.ExtraByProvider[provider]["externalid"])

		// Apply filter.
		if nameFilter != "" && !strings.EqualFold(userName, nameFilter) {
			continue
		}

		resource := map[string]any{
			"schemas":    []string{UserSchemaID},
			"id":         user.Name,
			"userName":   userName,
			"externalId": externalID,
			"active":     user.Enabled == nil || (user.Enabled != nil && *user.Enabled),
			"meta": map[string]any{
				"resourceType": UserResource,
				"created":      user.CreationTimestamp,
				"location":     locationURL(r, provider, UserEndpoint, user.Name),
			},
		}

		primaryEmail := first(attr.ExtraByProvider[provider]["email"])
		if primaryEmail != "" {
			resource["emails"] = []map[string]any{
				{
					"value":   primaryEmail,
					"primary": true,
				},
			}
		}

		allResources = append(allResources, resource)
	}

	totalResults := len(allResources)

	// Apply pagination.
	paginatedResources, startIndex := Paginate(allResources, pagination)
	if paginatedResources == nil {
		paginatedResources = []any{}
	}

	response := ListResponse{
		Schemas:      []string{ListSchemaID},
		Resources:    paginatedResources,
		TotalResults: totalResults,
		ItemsPerPage: len(paginatedResources),
		StartIndex:   startIndex,
	}

	writeResponse(w, response)
}

// CreateUser creates a new user.
// Returns:
//   - 201 on success
//   - 400 for invalid requests
//   - 409 if the user already exists.
func (s *SCIMServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::CreateUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Trace("scim::CreateUser: request body:", string(bodyBytes))

	payload := &SCIMUser{}

	err = json.Unmarshal(bodyBytes, payload)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	list, err := s.userCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to list users: %s", err)
		writeError(w, NewInternalError())
		return
	}

	for _, user := range list {
		attr, err := s.userAttributeCache.Get(user.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			logrus.Errorf("scim::CreateUser: failed to get user attributes for %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}

		userName := first(attr.ExtraByProvider[provider]["username"])
		if strings.EqualFold(userName, payload.UserName) {
			writeError(w, NewError(http.StatusConflict, fmt.Sprintf("User with username %s already exists", payload.UserName)))
			return
		}
	}

	principalName := provider + "_user://" + payload.UserName
	user, err := s.userMGR.EnsureUser(principalName, payload.UserName)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to ensure user %s: %s", principalName, err)
		writeError(w, NewInternalError())
		return
	}

	groupPrincipals := []v3.Principal{} // TBD
	extras := map[string][]string{
		"username":    {payload.UserName},
		"externalid":  {payload.ExternalID},
		"principalid": {principalName},
	}

	var primaryEmail string
	for _, email := range payload.Emails {
		if email.Primary {
			primaryEmail = email.Value
			extras["email"] = []string{primaryEmail}
			break
		}
	}

	err = s.userMGR.UserAttributeCreateOrUpdate(user.Name, provider, groupPrincipals, extras)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to ensure user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	response := map[string]any{
		"schemas":    []string{UserSchemaID},
		"id":         user.Name,
		"userName":   payload.UserName,
		"externalId": payload.ExternalID,
		"active":     user.Enabled == nil || (user.Enabled != nil && *user.Enabled),
		"meta": map[string]any{
			"resourceType": UserResource,
			"created":      user.CreationTimestamp,
			"location":     locationURL(r, provider, UserEndpoint, user.Name),
		},
	}
	if primaryEmail != "" {
		response["emails"] = []map[string]any{
			{
				"value":   primaryEmail,
				"primary": true,
			},
		}
	}

	writeResponse(w, response, http.StatusCreated)
}

// GetUser retrieves a user by their ID.
// Returns:
//   - 200 on success
//   - 400 for invalid requests
//   - 404 if the user is not found or is a system user.
func (s *SCIMServer) GetUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetUser: query %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		}

		logrus.Errorf("scim::GetUser: failed to get user: %s", err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
		return
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		logrus.Errorf("scim::ListUsers: failed to get user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	response := map[string]any{
		"schemas":    []string{UserSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     user.Enabled == nil || (user.Enabled != nil && *user.Enabled),
		"meta": map[string]any{
			"resourceType": UserResource,
			"created":      user.CreationTimestamp,
			"location":     locationURL(r, provider, UserEndpoint, user.Name),
		},
	}

	primaryEmail := first(attr.ExtraByProvider[provider]["email"])
	if primaryEmail != "" {
		response["emails"] = []map[string]any{
			{
				"value":   primaryEmail,
				"primary": true,
			},
		}
	}

	writeResponse(w, response)
}

// UpdateUser updates an existing user.
// Returns:
//   - 200 on success
//   - 400 for invalid requests
//   - 404 if the user is not found or is a system user
//   - 409 if attempting to deprovision the default admin user.
func (s *SCIMServer) UpdateUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::UpdateUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::UpdateUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	logrus.Trace("scim::UpdateUser: request body:", string(bodyBytes))

	payload := &SCIMUser{}

	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(payload)
	if err != nil {
		logrus.Errorf("scim::UpdateUser: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("User %s not found", id)))
			return
		}

		logrus.Errorf("scim::UpdateUser: failed to get user: %s", err)
		writeError(w, NewInternalError())
		return
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		logrus.Errorf("scim::UpdateUsers: failed to get user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
		return
	}

	userIsActive := user.Enabled == nil || (user.Enabled != nil && *user.Enabled)

	var shouldUpdateAttr, shouldUpdateUser bool
	attr = attr.DeepCopy()
	if attr.ExtraByProvider[provider] == nil {
		attr.ExtraByProvider[provider] = map[string][]string{}
	}
	if userName := first(attr.ExtraByProvider[provider]["username"]); userName != payload.UserName {
		attr.ExtraByProvider[provider]["username"] = []string{payload.UserName}
		shouldUpdateAttr = true
	}
	if externalId := first(attr.ExtraByProvider[provider]["externalid"]); externalId != payload.ExternalID {
		attr.ExtraByProvider[provider]["externalid"] = []string{payload.ExternalID}
		shouldUpdateAttr = true
	}
	if userIsActive != payload.Active {
		if user.IsDefaultAdmin() && !payload.Active {
			writeError(w, NewError(http.StatusConflict, "Cannot deprovision default admin user"))
			return
		}

		user = user.DeepCopy()
		user.Enabled = &payload.Active
		shouldUpdateUser = true
	}
	if shouldUpdateAttr {
		if attr, err = s.userAttributes.Update(attr); err != nil {
			logrus.Errorf("scim::UpdateUser: failed to update user attributes for %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}
	}
	if shouldUpdateUser {
		if _, err = s.users.Update(user); err != nil {
			logrus.Errorf("scim::UpdateUser: failed to update user %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}
	}

	response := map[string]any{
		"schemas":    []string{UserSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     payload.Active,
		"meta": map[string]any{
			"resourceType": UserResource,
			"created":      user.CreationTimestamp,
			"location":     locationURL(r, provider, UserEndpoint, user.Name),
		},
	}

	primaryEmail := first(attr.ExtraByProvider[provider]["email"])
	if primaryEmail != "" {
		response["emails"] = []map[string]any{
			{
				"value":   primaryEmail,
				"primary": true,
			},
		}
	}

	writeResponse(w, response)
}

// DeleteUser permanently deletes a user by ID.
// Returns:
//   - 204 on successful deletion
//   - 404 if the user is not found or is a system user
//   - 409 if attempting to delete the default admin user.
func (s *SCIMServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::DeleteUser: url %s", r.URL.String())
	// provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		}

		logrus.Errorf("scim::DeleteUser: failed to get user: %s", err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
		return
	}

	if user.IsDefaultAdmin() {
		writeError(w, NewError(http.StatusConflict, "Cannot delete default admin user"))
		return
	}

	// attr, err := s.userAttributeCache.Get(user.Name)
	// if err != nil {
	// 	logrus.Errorf("scim::DeleteUser: failed to get user attributes for %s: %s", user.Name, err)
	// 	writeError(w, NewInternalError())
	// 	return
	// }

	// if attrUserName := first(attr.ExtraByProvider[provider]["username"]); attrUserName == "" {
	// 	writeError(w, NewError(http.StatusBadRequest, "user not found"))
	// 	return
	// }

	if err := s.users.Delete(user.Name, &metav1.DeleteOptions{}); err != nil {
		logrus.Errorf("scim::DeleteUser: failed to delete user %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	writeResponse(w, noPayload, http.StatusNoContent)
}

// PatchUser applies partial modifications to a user.
// Currently supports the "replace" operation for updating active status, externalId,
// and primary email address.
// Returns:
//   - 200 on success
//   - 400 for invalid requests
//   - 404 if the user is not found or is a system user
//   - 409 if attempting to deprovision the default admin user.
func (s *SCIMServer) PatchUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::PatchUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		}

		logrus.Errorf("scim::PatchUser: failed to get user: %s", err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::PatchUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Trace("scim::PatchUser: request body:", string(bodyBytes))

	payload := struct {
		Operations []patchOp `json:"Operations"`
		Schemas    []string  `json:"schemas"`
	}{}

	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::PatchUser: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		logrus.Errorf("scim::PatchUser: failed to get user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusConflict, "Cannot update system or default admin user"))
		return
	}

	attr = attr.DeepCopy()
	user = user.DeepCopy()

	var shouldUpdateAttr, shouldUpdateUser bool
	for _, op := range payload.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			updateAttr, updateUser, err := applyReplaceUser(provider, attr, user, op)
			if err != nil {
				logrus.Errorf("scim::PatchUser: failed to apply replace operation: %s", err)
				writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Failed to apply replace operation: %s", err)))
				return
			}
			if updateAttr {
				shouldUpdateAttr = true
			}
			if updateUser {
				shouldUpdateUser = true
			}
		default:
			writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported patch operation: %s", op.Op)))
			return
		}
	}

	if shouldUpdateAttr {
		if attr, err = s.userAttributes.Update(attr); err != nil {
			logrus.Errorf("scim::PatchUser: failed to update user attributes for %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}
	}
	if shouldUpdateUser {
		if _, err = s.users.Update(user); err != nil {
			logrus.Errorf("scim::PatchUser: failed to update user %s: %s", user.Name, err)
			writeError(w, NewInternalError())
			return
		}
	}

	response := map[string]any{
		"schemas":    []string{UserSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     user.Enabled == nil || (user.Enabled != nil && *user.Enabled),
		"meta": map[string]any{
			"resourceType": UserResource,
			"created":      user.CreationTimestamp,
			"location":     locationURL(r, provider, UserEndpoint, user.Name),
		},
	}

	primaryEmail := first(attr.ExtraByProvider[provider]["email"])
	if primaryEmail != "" {
		response["emails"] = []map[string]any{
			{
				"value":   primaryEmail,
				"primary": true,
			},
		}
	}

	writeResponse(w, response)
}

// applyReplaceUser applies a SCIM PATCH replace operation to a user.
func applyReplaceUser(provider string, attr *v3.UserAttribute, user *v3.User, op patchOp) (bool, bool, error) {
	if op.Path == "" {
		fields, ok := op.Value.(map[string]any)
		if !ok {
			return false, false, fmt.Errorf("invalid value type for replace operation: %T", op.Value)
		}

		var shouldUpdateAttr, shouldUpdateUser bool
		for name, value := range fields {
			updateAttr, updateUser, err := applyReplaceUser(provider, attr, user, patchOp{
				Op:    "replace",
				Path:  name,
				Value: value,
			})
			if err != nil {
				return false, false, fmt.Errorf("failed to apply replace operation: %v", err)
			}
			if updateAttr {
				shouldUpdateAttr = true
			}
			if updateUser {
				shouldUpdateUser = true
			}
		}
		return shouldUpdateAttr, shouldUpdateUser, nil
	}

	var updateAttr, updateUser bool
	switch strings.ToLower(op.Path) {
	case "active":
		active, ok := op.Value.(bool)
		if !ok {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for active: %v", op.Value))
		}

		userIsActive := user.Enabled == nil || (user.Enabled != nil && *user.Enabled)
		if userIsActive != active {
			if user.IsDefaultAdmin() && !active {
				return false, false, NewError(http.StatusConflict, "Cannot deprovision default admin user")
			}

			user.Enabled = &active
			updateUser = true
		}
	case "externalid":
		externalID, ok := op.Value.(string)
		if !ok {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for externalId: %v", op.Value))
		}

		if first(attr.ExtraByProvider[provider]["externalid"]) != externalID {
			attr.ExtraByProvider[provider]["externalid"] = []string{externalID}
			updateAttr = true
		}
	case "emails[primary eq true].value": // Primary email.
		email, ok := op.Value.(string)
		if !ok {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for email: %v", op.Value))
		}

		if first(attr.ExtraByProvider[provider]["email"]) != email {
			attr.ExtraByProvider[provider]["email"] = []string{email}
			updateAttr = true
		}
	default:
		return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported patch path: %s", op.Path))
	}

	return updateAttr, updateUser, nil
}
