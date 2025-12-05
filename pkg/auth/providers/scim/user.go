package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Bool bool

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

func (s *SCIMServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::ListUsers: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	var (
		nameFilter        string
		startIndex, count int
		err               error
	)
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

	if value := r.URL.Query().Get("startIndex"); value != "" {
		startIndex, err = strconv.Atoi(value)
		if err != nil {
			writeError(w, NewError(http.StatusBadRequest, "Invalid startIndex"))
			return
		}
	}
	if value := r.URL.Query().Get("count"); value != "" {
		count, err = strconv.Atoi(value)
		if err != nil {
			writeError(w, NewError(http.StatusBadRequest, "Invalid count"))
			return
		}
	}

	logrus.Infof("scim::ListUsers: userName=%s, startIndex=%d, count=%d", nameFilter, startIndex, count)

	list, err := s.userCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("scim::ListUsers: failed to list users: %s", err)
		writeError(w, NewInternalError())
		return
	}

	var resources []any
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

		if nameFilter == "" {
			resources = append(resources, resource)
		} else if nameFilter != "" && strings.EqualFold(userName, nameFilter) {
			resources = append(resources, resource)
			break
		}
	}
	if resources == nil {
		resources = []any{}
	}

	response := ListResponse{
		Schemas:      []string{ListSchemaID},
		Resources:    resources,
		TotalResults: len(resources), // No pagination for now.
		ItemsPerPage: len(resources),
	}

	if response.TotalResults > 0 {
		response.StartIndex = 1
	}

	writeResponse(w, response)
}

func (s *SCIMServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::CreateUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::CreateUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Info("scim::CreateUser: request body:", string(bodyBytes))

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
			} else {
				logrus.Errorf("scim::CreateUser: failed to get user attributes for %s: %s", user.Name, err)
				writeError(w, NewInternalError())
				return
			}
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

func (s *SCIMServer) GetUser(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::GetUser: query %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		} else {
			logrus.Errorf("scim::GetUser: failed to get user: %s", err)
			writeError(w, NewInternalError())
		}
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

func (s *SCIMServer) UpdateUser(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::UpdateUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::UpdateUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Info("scim::UpdateUser: request body:", string(bodyBytes))

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
		} else {
			logrus.Errorf("scim::UpdateUser: failed to get user: %s", err)
			writeError(w, NewInternalError())
		}
		return
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		logrus.Errorf("scim::UpdateUsers: failed to get user attributes for %s: %s", user.Name, err)
		writeError(w, NewInternalError())
		return
	}

	if user.IsSystem() {
		writeError(w, NewError(http.StatusConflict, "Cannot update system or default admin user"))
		return
	}

	userIsActive := user.Enabled == nil || (user.Enabled != nil && *user.Enabled)

	// Handle delete on deactivation.
	// To make this possible we need to ensure that update is idempotent:
	// - don't fail if payload.Active is false and user doesn't exist. Parrot the payload back.

	// if userIsActive && !payload.Active && settings.SCIMDeleteDeactivatedUsers.Get() == "true" {
	// 	if err := s.users.Delete(user.Name, &metav1.DeleteOptions{}); err != nil {
	// 		logrus.Errorf("scim::UpdateUser: failed to delete deactivated user %s: %s", user.Name, err)
	// 		writeError(w, NewInternalError())
	// 		return
	// 	}
	// }

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

func (s *SCIMServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::DeleteUser: url %s", r.URL.String())

	// provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		} else {
			logrus.Errorf("scim::DeleteUser: failed to get user: %s", err)
			writeError(w, NewInternalError())
		}
		return
	}

	if user.IsSystem() || user.IsDefaultAdmin() {
		writeError(w, NewError(http.StatusConflict, "Cannot delete system or default admin user"))
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

func (s *SCIMServer) PatchUser(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::PatchUser: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	user, err := s.userCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("User %s not found", id)))
			return
		} else {
			logrus.Errorf("scim::PatchUser: failed to get user: %s", err)
			writeError(w, NewInternalError())
		}
		return
	}

	if user.IsSystem() || user.IsDefaultAdmin() {
		writeError(w, NewError(http.StatusConflict, "Cannot delete system or default admin user"))
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::PatchUser: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Info("scim::PatchUser: request body:", string(bodyBytes))

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
			user.Enabled = &active
			updateUser = true
		}
	case "externalId":
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
