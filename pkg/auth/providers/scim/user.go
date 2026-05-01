package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Bool represents a boolean value that can be unmarshaled from JSON strings or boolean literals.
// Okta uses boolean values, whereas Azure uses strings "true"/"false" e.g. for primary email flag.
type Bool bool

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (b *Bool) UnmarshalJSON(data []byte) error {
	switch strings.ToLower(strings.Trim(string(data), `"`)) {
	case "true":
		*b = true
	case "false":
		*b = false
	default:
		return fmt.Errorf("invalid boolean value: %s", data)
	}

	return nil
}

func (b Bool) Bool() bool {
	return bool(b)
}

// boolFromValue converts an [any] value to bool, accepting both actual bool
// values (e.g. Okta) and string representations "true"/"false" (e.g. Azure),
// by constructing the raw JSON bytes and delegating to [Bool.UnmarshalJSON].
func boolFromValue(v any) (bool, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return false, err
	}

	var b Bool
	if err := b.UnmarshalJSON(data); err != nil {
		return false, err
	}

	return b.Bool(), nil
}

// scimUser represents a SCIM user.
type scimUser struct {
	Schemas    []string `json:"schemas"`    // Resource schema URIs.
	ID         string   `json:"id"`         // Service provider identifier User.Name.
	ExternalID string   `json:"externalId"` // IdPs identifier.
	Active     Bool     `json:"active"`     // A flag indicating the user's active status.
	Name       struct { // The components of the user's real name.
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	DisplayName string     `json:"displayName"` // The name of the user, suitable for display to end-users.
	UserName    string     `json:"userName"`    // A service provider's unique identifier for the user, typically used by the user to directly authenticate to the service provider.
	Emails      []struct { // Email addresses for the user.
		Value   string `json:"value"`
		Primary Bool   `json:"primary"`
	} `json:"emails"`
	Meta Meta `json:"meta"` // The resource metadata.
}

// ListUsers returns a list of users.
// It supports filtering by userName using the "eq" operator.
// Pagination is supported via startIndex (1-based) and count query parameters.
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::ListUsers: url %s", r.URL)

	provider := r.PathValue("provider")

	// Parse pagination parameters.
	pagination, err := parsePaginationParams(r)
	if err != nil {
		writeError(w, NewError(http.StatusBadRequest, err.Error()))
		return
	}

	// Parse filter.
	var filter *Filter
	if value := r.URL.Query().Get("filter"); value != "" {
		filter, err = ParseFilter(value)
		if err != nil {
			writeError(w, NewError(http.StatusBadRequest, err.Error()))
			return
		}
		if err := filter.ValidateForAttributes([]string{"userName", "externalId"}, opEqual); err != nil {
			writeError(w, NewError(http.StatusBadRequest, err.Error()))
			return
		}
	}

	var filterValue string
	if filter != nil {
		filterValue = filter.Value
	}
	logrus.Tracef("scim::ListUsers: userName=%s, startIndex=%d, count=%d", filterValue, pagination.startIndex, pagination.count)

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
			if apierrors.IsNotFound(err) {
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
		var filterTarget string
		if filter != nil && strings.EqualFold(filter.Attribute, "externalId") {
			filterTarget = externalID
		} else {
			filterTarget = userName
		}
		if !filter.Matches(filterTarget) {
			continue
		}

		resource := map[string]any{
			"schemas":    []string{userSchemaID},
			"id":         user.Name,
			"userName":   userName,
			"externalId": externalID,
			"active":     user.GetEnabled(),
			"meta": map[string]any{
				"resourceType": userResource,
				"created":      user.CreationTimestamp,
				"location":     locationURL(r, provider, userEndpoint, user.Name),
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
	paginatedResources, startIndex := paginate(allResources, pagination)
	if paginatedResources == nil {
		paginatedResources = []any{}
	}

	response := listResponse{
		Schemas:      []string{listSchemaID},
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
//
// Note: The requested state of the active attribute is deliberately ignored - true is implied.
// For IdP-driven provisioning, active acts as a lifecycle signal:
// - active=true -> "this user exists and has access, provision them"
// - active=false -> "this user no longer has access, deprovision them" (via PATCH/PUT).
// We could reject such requests, but it could potentially break poorly-behaved IdPs.
func (s *SCIMServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::CreateUser: url %s", r.URL)

	provider := r.PathValue("provider")

	payload := scimUser{}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if payload.UserName == "" {
		writeError(w, NewError(http.StatusBadRequest, "userName is required"))
		return
	}

	cfg := s.getConfig(provider)
	uid := cfg.userID(payload)
	if uid == "" {
		writeError(w, NewError(http.StatusBadRequest,
			fmt.Sprintf("%s is required when configured as userIdAttribute", cfg.UserIDAttribute)))
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

	principalName := userPrincipalName(provider, uid)
	displayName := payload.DisplayName
	if displayName == "" {
		displayName = payload.UserName
	}
	user, err := s.userMGR.EnsureUser(principalName, displayName)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to ensure user %s: %s", principalName, err)
		writeError(w, NewInternalError())
		return
	}

	groupPrincipals := []v3.Principal{}
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

	location := locationURL(r, provider, userEndpoint, user.Name)
	response := map[string]any{
		"schemas":    []string{userSchemaID},
		"id":         user.Name,
		"userName":   payload.UserName,
		"externalId": payload.ExternalID,
		"active":     user.GetEnabled(),
		"meta": map[string]any{
			"resourceType": userResource,
			"created":      user.CreationTimestamp,
			"location":     location,
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

	w.Header().Set("Location", location)
	writeResponse(w, response, http.StatusCreated)
}

// GetUser retrieves a user by their ID.
// Returns:
//   - 200 on success
//   - 400 for invalid requests
//   - 404 if the user is not found or is a system user.
func (s *SCIMServer) GetUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetUser: query %s", r.URL)

	provider := r.PathValue("provider")
	id := r.PathValue("id")

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
		logrus.Errorf("scim::GetUsers: failed to get user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	response := map[string]any{
		"schemas":    []string{userSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     user.GetEnabled(),
		"meta": map[string]any{
			"resourceType": userResource,
			"created":      user.CreationTimestamp,
			"location":     locationURL(r, provider, userEndpoint, user.Name),
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
	logrus.Tracef("scim::UpdateUser: url %s", r.URL)

	provider := r.PathValue("provider")
	id := r.PathValue("id")

	payload := scimUser{}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::UpdateUser: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if payload.UserName == "" {
		writeError(w, NewError(http.StatusBadRequest, "userName is required"))
		return
	}

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
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

	cfg := s.getConfig(provider)

	var shouldUpdateAttr, shouldUpdateUser bool
	attr = attr.DeepCopy()
	if attr.ExtraByProvider[provider] == nil {
		attr.ExtraByProvider[provider] = map[string][]string{}
	}
	if userName := first(attr.ExtraByProvider[provider]["username"]); userName != payload.UserName {
		if cfg.UserIDAttribute != UserIDExternalID {
			writeError(w, NewError(http.StatusBadRequest, "userName cannot be changed when it is used as the principal identifier"))
			return
		}
		attr.ExtraByProvider[provider]["username"] = []string{payload.UserName}
		shouldUpdateAttr = true
	}
	if externalId := first(attr.ExtraByProvider[provider]["externalid"]); externalId != payload.ExternalID {
		attr.ExtraByProvider[provider]["externalid"] = []string{payload.ExternalID}
		shouldUpdateAttr = true
	}

	payloadActive := payload.Active.Bool()
	if user.GetEnabled() != payloadActive {
		if user.IsDefaultAdmin() && !payloadActive {
			writeError(w, NewError(http.StatusConflict, "Cannot deprovision default admin user"))
			return
		}

		user = user.DeepCopy()
		user.Enabled = &payloadActive
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

	location := locationURL(r, provider, userEndpoint, user.Name)
	response := map[string]any{
		"schemas":    []string{userSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     payloadActive,
		"meta": map[string]any{
			"resourceType": userResource,
			"created":      user.CreationTimestamp,
			"location":     location,
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

	w.Header().Set("Location", location)
	writeResponse(w, response)
}

// DeleteUser permanently deletes a user by ID.
// Returns:
//   - 204 on successful deletion
//   - 404 if the user is not found or is a system user
//   - 409 if attempting to delete the default admin user.
func (s *SCIMServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::DeleteUser: url %s", r.URL)
	// provider := r.PathValue("provider")
	id := r.PathValue("id")

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
	logrus.Tracef("scim::PatchUser: url %s", r.URL)

	provider := r.PathValue("provider")
	id := r.PathValue("id")

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

	payload := struct {
		Operations []patchOp `json:"Operations"`
		Schemas    []string  `json:"schemas"`
	}{}
	err = json.NewDecoder(r.Body).Decode(&payload)
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

	attr = attr.DeepCopy()
	user = user.DeepCopy()

	cfg := s.getConfig(provider)

	var shouldUpdateAttr, shouldUpdateUser bool
	for _, op := range payload.Operations {
		switch strings.ToLower(op.Op) {
		case "replace", "add":
			updateAttr, updateUser, err := applyPatchUser(provider, attr, user, op, cfg)
			if err != nil {
				logrus.Errorf("scim::PatchUser: failed to apply %s operation: %s", op.Op, err)
				writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Failed to apply %s operation: %s", op.Op, err)))
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

	location := locationURL(r, provider, userEndpoint, user.Name)
	response := map[string]any{
		"schemas":    []string{userSchemaID},
		"id":         user.Name,
		"userName":   first(attr.ExtraByProvider[provider]["username"]),
		"externalId": first(attr.ExtraByProvider[provider]["externalid"]),
		"active":     user.GetEnabled(),
		"meta": map[string]any{
			"resourceType": userResource,
			"created":      user.CreationTimestamp,
			"location":     location,
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

	w.Header().Set("Location", location)
	writeResponse(w, response)
}

// applyPatchUser applies a SCIM PATCH add/replace operation to a user.
// For single-valued attributes, add and replace have identical semantics (RFC 7644 §3.5.2).
func applyPatchUser(provider string, attr *v3.UserAttribute, user *v3.User, op patchOp, cfg providerConfig) (bool, bool, error) {
	if op.Path == "" {
		fields, ok := op.Value.(map[string]any)
		if !ok {
			return false, false, fmt.Errorf("invalid value type for replace operation: %T", op.Value)
		}

		var shouldUpdateAttr, shouldUpdateUser bool
		for name, value := range fields {
			updateAttr, updateUser, err := applyPatchUser(provider, attr, user, patchOp{
				Op:    "replace",
				Path:  name,
				Value: value,
			}, cfg)
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

	path, _, err := stripSchemaURN(op.Path, userResource)
	if err != nil {
		return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid path %q: %s", op.Path, err))
	}

	var updateAttr, updateUser bool
	switch strings.ToLower(path) {
	case "active":
		active, err := boolFromValue(op.Value)
		if err != nil {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for active: %v", op.Value))
		}

		if user.GetEnabled() != active {
			if user.IsDefaultAdmin() && !active {
				return false, false, NewError(http.StatusConflict, "Cannot deprovision default admin user")
			}

			user.Enabled = &active
			updateUser = true
		}
	case "displayname":
		displayName, ok := op.Value.(string)
		if !ok {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for displayName: %v", op.Value))
		}

		if user.DisplayName != displayName {
			user.DisplayName = displayName
			updateUser = true
		}
	case "username":
		if cfg.UserIDAttribute != UserIDExternalID {
			return false, false, NewError(http.StatusBadRequest, "userName cannot be changed when it is used as the principal identifier")
		}

		username, ok := op.Value.(string)
		if !ok {
			return false, false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for userName: %v", op.Value))
		}

		if first(attr.ExtraByProvider[provider]["username"]) != username {
			attr.ExtraByProvider[provider]["username"] = []string{username}
			updateAttr = true
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
