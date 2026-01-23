package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SCIMMember represents a member of a SCIM group.
type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

// SCIMGroup represents a SCIM group.
type SCIMGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id"`
	DisplayName string       `json:"displayName"`
	ExternalID  string       `json:"externalId"`
	Members     []SCIMMember `json:"members"`
}

// ListGroups returns a list of groups.
// It supports filtering by displayName using the "eq" operator.
// Pagination is supported via startIndex (1-based) and count query parameters.
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) ListGroups(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::ListGroups: url %s", r.URL)

	provider := mux.Vars(r)["provider"]

	// Parse pagination parameters.
	pagination, err := parsePaginationParams(r)
	if err != nil {
		writeError(w, NewError(http.StatusBadRequest, err.Error()))
		return
	}

	// Parse filter and excludedAttributes.
	var filter *Filter
	var excludeMembers bool
	if value := r.URL.Query().Get("filter"); value != "" {
		var err error
		filter, err = ParseFilter(value)
		if err != nil {
			writeError(w, NewError(http.StatusBadRequest, err.Error()))
			return
		}
		// Currently only support displayName eq "<value>" filter.
		if err := filter.ValidateForAttribute("displayName", OpEqual); err != nil {
			writeError(w, NewError(http.StatusBadRequest, err.Error()))
			return
		}
	}
	if value := r.URL.Query().Get("excludedAttributes"); value != "" {
		fields := strings.Split(value, ",")
		excludeMembers = slices.Contains(fields, "members")
	}

	var filterValue string
	if filter != nil {
		filterValue = filter.Value
	}
	logrus.Tracef("scim::ListGroups: displayName=%s, startIndex=%d, count=%d", filterValue, pagination.startIndex, pagination.count)

	groups, err := s.groupsCache.List(labels.Set{authProviderLabel: provider}.AsSelector())
	if err != nil {
		logrus.Errorf("scim::ListGroups: failed to list groups for provider %s: %s", provider, err)
		writeError(w, NewInternalError())
		return
	}

	// Sort groups by Name for deterministic ordering across pagination requests.
	// Without sorting, the cache order is undefined and pagination would be inconsistent.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	// Collect all matching resources (needed to compute totalResults).
	var allResources []any
	if len(groups) > 0 {
		var uniqueGroups map[string][]SCIMMember
		if !excludeMembers {
			uniqueGroups, err = s.getAllRancherGroupMembers(provider)
			if err != nil {
				logrus.Errorf("scim::ListGroups: %s", err)
				writeError(w, NewInternalError())
				return
			}
		}

		for _, group := range groups {
			// Case insensitive match for displayName.
			if !filter.Matches(group.DisplayName) {
				continue
			}

			resource := map[string]any{
				"schemas":     []string{GroupSchemaID},
				"id":          group.Name,
				"displayName": group.DisplayName,
				"externalId":  group.ExternalID,
				"meta": map[string]any{
					"created":      group.CreationTimestamp,
					"resourceType": GroupResource,
					"location":     locationURL(r, provider, GroupEndpoint, group.Name),
				},
			}
			members, ok := uniqueGroups[group.DisplayName]
			if !ok {
				members = []SCIMMember{}
			}
			if !excludeMembers {
				resource["members"] = members
			}

			allResources = append(allResources, resource)
		}
	}

	totalResults := len(allResources)

	// Apply pagination.
	paginatedResources, startIndex := paginate(allResources, pagination)
	if paginatedResources == nil {
		paginatedResources = []any{}
	}

	response := &ListResponse{
		Schemas:      []string{ListSchemaID},
		Resources:    paginatedResources,
		TotalResults: totalResults,
		ItemsPerPage: len(paginatedResources),
		StartIndex:   startIndex,
	}

	writeResponse(w, response)
}

// CreateGroup creates a group.
// Returns:
//   - 201 on success
//   - 400 for invalid requests.
func (s *SCIMServer) CreateGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::CreateGroup: url %s", r.URL)

	provider := mux.Vars(r)["provider"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Tracef("scim::CreateGroup: request body: %s", bodyBytes)

	payload := SCIMGroup{}
	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to unmarshal request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	group, created, err := s.ensureRancherGroup(provider, payload)
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to ensure rancher group: %s", err)
		writeError(w, NewInternalError())
		return
	}

	if !created {
		writeError(w, NewError(http.StatusConflict,
			fmt.Sprintf("Group with displayName %q already exists", payload.DisplayName), "uniqueness"))
		return
	}

	if len(payload.Members) > 0 {
		err = s.syncGroupMembers(provider, group.DisplayName, payload.Members)
		if err != nil {
			logrus.Errorf("scim::CreateGroup: failed to sync group members: %s", err)
			writeError(w, NewInternalError())
			return
		}
	}

	resource := map[string]any{
		"schemas":     []string{GroupSchemaID},
		"id":          group.Name,
		"displayName": group.DisplayName,
		"externalId":  group.ExternalID,
		"meta": map[string]any{
			"created":      group.CreationTimestamp,
			"resourceType": GroupResource,
			"location":     locationURL(r, provider, GroupEndpoint, group.Name),
		},
	}
	members := payload.Members
	if members == nil {
		members = []SCIMMember{}
	}
	resource["members"] = members

	writeResponse(w, resource, http.StatusCreated)
}

// GetGroup returns a group by ID.
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) GetGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetGroup: url %s", r.URL)

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	var excludeMembers bool
	if value := r.URL.Query().Get("excludedAttributes"); value != "" {
		fields := strings.Split(value, ",")
		excludeMembers = slices.Contains(fields, "members")
	}

	group, err := s.groupsCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Group %s not found", id)))
			return
		}
		logrus.Errorf("scim::GetGroup: failed to get group %s: %s", id, err)
		writeError(w, NewInternalError())
		return
	}

	var members []SCIMMember
	if !excludeMembers {
		members, err = s.getRancherGroupMembers(provider, group.DisplayName)
		if err != nil {
			logrus.Errorf("scim::GetGroups: %s", err)
			writeError(w, NewInternalError())
			return
		}
	}

	resource := map[string]any{
		"schemas":     []string{GroupSchemaID},
		"id":          group.Name,
		"displayName": group.DisplayName,
		"externalId":  group.ExternalID,
		"meta": map[string]any{
			"created":      group.CreationTimestamp,
			"resourceType": GroupResource,
			"location":     locationURL(r, provider, GroupEndpoint, group.Name),
		},
	}
	if members == nil {
		members = []SCIMMember{}
	}
	if !excludeMembers {
		resource["members"] = members
	}

	writeResponse(w, resource)
}

// UpdateGroup updates a group.
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::UpdateGroup: url %s", r.URL)

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Tracef("scim::UpdateGroup: request body: %s", bodyBytes)

	payload := SCIMGroup{}
	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to unmarshal request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if id != payload.ID {
		logrus.Errorf("scim::UpdateGroup: id in URL %s does not match id in body %s", id, payload.ID)
		writeError(w, NewError(http.StatusBadRequest, "Mismatched Group id"))
		return
	}

	group, _, err := s.ensureRancherGroup(provider, payload)
	if err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to ensure rancher group %s: %s", id, err)
		writeError(w, NewInternalError())
		return
	}

	err = s.syncGroupMembers(provider, group.DisplayName, payload.Members)
	if err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to sync group members for %s: %s", id, err)
		writeError(w, NewInternalError())
		return
	}

	resource := map[string]any{
		"schemas":     []string{GroupSchemaID},
		"id":          group.Name,
		"displayName": group.DisplayName,
		"externalId":  group.ExternalID,
		"meta": map[string]any{
			"created":      group.CreationTimestamp,
			"resourceType": GroupResource,
			"location":     locationURL(r, provider, GroupEndpoint, group.Name),
		},
	}
	members := payload.Members
	if members == nil {
		members = []SCIMMember{}
	}
	resource["members"] = members

	writeResponse(w, resource)
}

// PatchGroup applies partial modifications to a group.
// Currently supports
//   - the "replace" operation for updating externalId.
//   - the "add" and "remove" operations for managing group members.
//
// Returns:
//   - 200 on success
//   - 400 for invalid requests.
func (s *SCIMServer) PatchGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::PatchGroup: url %s", r.URL)

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	group, err := s.groupsCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Group %s not found", id)))
			return
		}

		logrus.Errorf("scim::PatchGroup: failed to get group: %s", err)
		writeError(w, NewInternalError())
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::PatchGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	logrus.Tracef("scim::PatchGroup: request body: %s", bodyBytes)

	payload := struct {
		Operations []patchOp `json:"Operations"`
		Schemas    []string  `json:"schemas"`
	}{}

	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload)
	if err != nil {
		logrus.Errorf("scim::PatchGroup: failed to decode request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	group = group.DeepCopy()
	var shouldUpdateGroup bool

	var membersToAdd []SCIMMember
	var membersToRemove []string

	for _, op := range payload.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			updated, err := applyReplaceGroup(group, op)
			if err != nil {
				logrus.Errorf("scim::PatchGroup: failed to apply replace operation: %s", err)
				writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Failed to apply replace operation: %s", err)))
				return
			}
			if updated {
				shouldUpdateGroup = true
			}

		case "add":
			// Add members to group
			if strings.ToLower(op.Path) != "members" {
				writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported add path: %s", op.Path)))
				return
			}

			members, ok := op.Value.([]any)
			if !ok {
				writeError(w, NewError(http.StatusBadRequest, "Invalid members value for add operation"))
				return
			}
			for _, m := range members {
				memberMap, ok := m.(map[string]any)
				if !ok {
					continue
				}
				value, _ := memberMap["value"].(string)
				display, _ := memberMap["display"].(string)
				if value != "" {
					membersToAdd = append(membersToAdd, SCIMMember{
						Value:   value,
						Display: display,
					})
				}
			}
		case "remove":
			// Remove members from group
			if !strings.HasPrefix(strings.ToLower(op.Path), "members[") {
				writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported remove path: %s", op.Path)))
				return
			}
			// Format: members[value eq "user-id"]
			// Extract user-id from the filter
			if userID := extractMemberValueFromPath(op.Path); userID != "" {
				membersToRemove = append(membersToRemove, userID)
			} else {
				writeError(w, NewError(http.StatusBadRequest, "Invalid member removal path format"))
				return
			}
		default:
			writeError(w, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported patch operation: %s", op.Op)))
			return
		}
	}

	// Apply group updates
	if shouldUpdateGroup {
		if group, err = s.groups.Update(group); err != nil {
			logrus.Errorf("scim::PatchGroup: failed to update group %s: %s", group.Name, err)
			writeError(w, NewInternalError())
			return
		}
	}

	// Apply member additions
	for _, member := range membersToAdd {
		if err := s.addGroupMember(provider, group.DisplayName, member); err != nil {
			logrus.Errorf("scim::PatchGroup: failed to add member %s: %s", member.Value, err)
			writeError(w, NewInternalError())
			return
		}
	}

	// Apply member removals
	for _, memberValue := range membersToRemove {
		if err := s.removeGroupMember(provider, group.DisplayName, memberValue); err != nil {
			logrus.Errorf("scim::PatchGroup: failed to remove member %s: %s", memberValue, err)
			writeError(w, NewInternalError())
			return
		}
	}

	// Fetch current members for response
	members, err := s.getRancherGroupMembers(provider, group.DisplayName)
	if err != nil {
		logrus.Errorf("scim::PatchGroup: failed to get group members: %s", err)
		writeError(w, NewInternalError())
		return
	}

	resource := map[string]any{
		"schemas":     []string{GroupSchemaID},
		"id":          group.Name,
		"displayName": group.DisplayName,
		"externalId":  group.ExternalID,
		"members":     members,
		"meta": map[string]any{
			"created":      group.CreationTimestamp,
			"resourceType": GroupResource,
			"location":     locationURL(r, provider, GroupEndpoint, group.Name),
		},
	}

	writeResponse(w, resource)
}

// DeleteGroup deletes a group.
// Returns:
//   - 204 on successful deletion
//   - 404 if the group is not found
func (s *SCIMServer) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::DeleteGroup: url %s", r.URL)

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	group, err := s.groupsCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Group %s not found", id)))
			return
		}

		logrus.Errorf("scim::DeleteGroup: failed to get group: %s", err)
		writeError(w, NewInternalError())
		return
	}

	err = s.removeAllGroupMembers(provider, group.DisplayName)
	if err != nil {
		logrus.Errorf("scim::DeleteGroup: failed to remove group members: %s", err)
		writeError(w, NewInternalError())
		return
	}

	if err := s.groups.Delete(group.Name, &metav1.DeleteOptions{}); err != nil {
		logrus.Errorf("scim::DeleteGroup: failed to delete group %s: %s", group.Name, err)
		writeError(w, NewInternalError())
		return
	}

	writeResponse(w, noPayload, http.StatusNoContent)
}

// getAllRancherGroupMembers retrieves all groups and their members for the specified provider.
func (s *SCIMServer) getAllRancherGroupMembers(provider string) (map[string][]SCIMMember, error) {
	list, err := s.userCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	uniqueGroups := make(map[string][]SCIMMember)
	for _, user := range list {
		if user.IsSystem() {
			continue
		}

		attr, err := s.userAttributeCache.Get(user.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // User has no attributes yet.
			}
			return nil, fmt.Errorf("failed to get user attributes for %s: %w", user.Name, err)
		}

		for _, group := range attr.GroupPrincipals[provider].Items {
			uniqueGroups[group.DisplayName] = append(uniqueGroups[group.DisplayName], SCIMMember{
				Value:   user.Name,
				Display: first(attr.ExtraByProvider[provider]["username"]),
			})
		}
	}

	return uniqueGroups, nil
}

// getRancherGroupMembers retrieves members of a specific group for the specified provider.
func (s *SCIMServer) getRancherGroupMembers(provider string, name string) ([]SCIMMember, error) {
	list, err := s.userCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var members []SCIMMember
	for _, user := range list {
		if user.IsSystem() {
			continue
		}

		attr, err := s.userAttributeCache.Get(user.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // User has no attributes yet.
			}
			return nil, fmt.Errorf("failed to get user attributes for %s: %w", user.Name, err)
		}

		for _, group := range attr.GroupPrincipals[provider].Items {
			if group.DisplayName == name {
				members = append(members, SCIMMember{
					Value:   user.Name,
					Display: first(attr.ExtraByProvider[provider]["username"]),
				})
				break // No need to check other groups.
			}
		}
	}

	return members, nil
}

// syncGroupMembers synchronizes the members of a group to match the provided list.
func (s *SCIMServer) syncGroupMembers(provider, groupName string, members []SCIMMember) error {
	rancherMembers, err := s.getRancherGroupMembers(provider, groupName)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	existing := make(map[string]string, len(rancherMembers))
	for _, m := range rancherMembers {
		existing[m.Value] = m.Display
	}

	for _, member := range members {
		if _, ok := existing[member.Value]; !ok {
			// New member added.
			err := s.addGroupMember(provider, groupName, member)
			if err != nil {
				return fmt.Errorf("failed to add member %s to group %s: %w", member.Value, groupName, err)
			}
		}
		delete(existing, member.Value)
	}

	for value := range existing {
		// Existing member removed.
		err := s.removeGroupMember(provider, groupName, value)
		if err != nil {
			return fmt.Errorf("failed to remove member %s from group %s: %w", value, groupName, err)
		}
	}

	return nil
}

// addGroupMember adds a member to a group.
func (s *SCIMServer) addGroupMember(provider, groupName string, member SCIMMember) error {
	user, err := s.userCache.Get(member.Value)
	if err != nil {
		return fmt.Errorf("failed to get user %s: %w", member.Value, err)
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		return fmt.Errorf("failed to get user attributes for %s: %w", user.Name, err)
	}

	for _, principal := range attr.GroupPrincipals[provider].Items {
		if principal.DisplayName == groupName {
			return nil // Member already exists.
		}
	}

	attr = attr.DeepCopy()
	if attr.GroupPrincipals == nil {
		attr.GroupPrincipals = make(map[string]v3.Principals)
	}
	principals := attr.GroupPrincipals[provider].Items
	principals = append(principals, v3.Principal{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s_group://%s", provider, groupName),
		},
		DisplayName:   groupName,
		MemberOf:      true,
		PrincipalType: "group",
		Provider:      provider,
	})

	attr.GroupPrincipals[provider] = v3.Principals{Items: principals}
	_, err = s.userAttributes.Update(attr)
	if err != nil {
		return fmt.Errorf("failed to update user attributes for %s: %w", user.Name, err)
	}

	return nil
}

func (s *SCIMServer) removeGroupMember(provider, groupName, value string) error {
	user, err := s.userCache.Get(value)
	if err != nil {
		return fmt.Errorf("failed to get user %s: %w", value, err)
	}

	attr, err := s.userAttributeCache.Get(user.Name)
	if err != nil {
		return fmt.Errorf("failed to get user attributes for %s: %w", user.Name, err)
	}

	if len(attr.GroupPrincipals[provider].Items) == 0 {
		return nil
	}

	attr = attr.DeepCopy()
	principals := attr.GroupPrincipals[provider].Items
	for i, principal := range principals {
		if principal.DisplayName == groupName {
			// Remove the principal.
			principals = append(principals[:i], principals[i+1:]...)
			break
		}
	}

	attr.GroupPrincipals[provider] = v3.Principals{Items: principals}
	_, err = s.userAttributes.Update(attr)
	if err != nil {
		return fmt.Errorf("failed to update user attributes for %s: %w", user.Name, err)
	}

	return nil
}

// removeAllGroupMembers removes all members from a group.
func (s *SCIMServer) removeAllGroupMembers(provider, groupName string) error {
	members, err := s.getRancherGroupMembers(provider, groupName)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	for _, member := range members {
		err := s.removeGroupMember(provider, groupName, member.Value)
		if err != nil {
			return fmt.Errorf("failed to remove member %s from group %s: %w", member.Value, groupName, err)
		}
	}

	return nil
}

// ensureRancherGroup ensures that a Rancher group exists for the given SCIM group.
// Returns the group, a boolean indicating if a new group was created, and any error.
func (s *SCIMServer) ensureRancherGroup(provider string, grp SCIMGroup) (*v3.Group, bool, error) {
	if grp.ID != "" {
		g, err := s.groupsCache.Get(grp.ID)
		return g, false, err
	}

	// Try to find an existing group by display name.
	groups, err := s.groupsCache.List(labels.Set{authProviderLabel: provider}.AsSelector())
	if err != nil {
		return nil, false, fmt.Errorf("failed to list groups for provider %s: %w", provider, err)
	}

	for _, g := range groups {
		if strings.EqualFold(g.DisplayName, grp.DisplayName) {
			// Found existing group - update if needed and return created=false.
			group := g.DeepCopy()
			if group.ExternalID != grp.ExternalID {
				group.ExternalID = grp.ExternalID
				updated, err := s.groups.Update(group)
				return updated, false, err
			}
			return g, false, nil
		}
	}

	// Create new group.
	group := &v3.Group{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grp-",
			Labels: map[string]string{
				authProviderLabel: provider,
			},
		},
		DisplayName: grp.DisplayName,
		Provider:    provider,
		ExternalID:  grp.ExternalID,
	}

	created, err := s.groups.Create(group)
	return created, true, err
}

// applyReplaceGroup applies a replace operation to a group.
// Currently only supports replacing externalId.
func applyReplaceGroup(group *v3.Group, op patchOp) (bool, error) {
	if op.Path == "" {
		// Bulk replace - replace multiple attributes at once
		fields, ok := op.Value.(map[string]any)
		if !ok {
			return false, fmt.Errorf("invalid value type for replace operation: %T", op.Value)
		}

		var updated bool
		for name, value := range fields {
			wasUpdated, err := applyReplaceGroup(group, patchOp{
				Op:    "replace",
				Path:  name,
				Value: value,
			})
			if err != nil {
				return false, fmt.Errorf("failed to apply replace operation: %v", err)
			}
			if wasUpdated {
				updated = true
			}
		}
		return updated, nil
	}

	var updated bool
	switch strings.ToLower(op.Path) {
	// Note: We can't change displayName as it is used as the unique identifier for groups.
	case "externalid":
		externalID, ok := op.Value.(string)
		if !ok {
			return false, NewError(http.StatusBadRequest, fmt.Sprintf("Invalid value for externalId: %v", op.Value))
		}
		if group.ExternalID != externalID {
			group.ExternalID = externalID
			updated = true
		}
	default:
		return false, NewError(http.StatusBadRequest, fmt.Sprintf("Unsupported patch path: %s", op.Path))
	}

	return updated, nil
}

// extractMemberValueFromPath extracts user ID from SCIM filter path like:
// members[value eq "user-123"] -> user-123
func extractMemberValueFromPath(path string) string {
	// Simple parser for: members[value eq "user-id"]
	start := strings.Index(path, `"`)
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(path, `"`)
	if end == -1 || end <= start {
		return ""
	}
	return path[start+1 : end]
}
