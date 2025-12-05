package scim

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

type SCIMGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id"`
	DisplayName string       `json:"displayName"`
	ExternalID  string       `json:"externalId"`
	Members     []SCIMMember `json:"members"`
}

func (s *SCIMServer) ListGroups(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::ListGroups: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	var (
		nameFilter        string
		excludeMembers    bool
		startIndex, count int
		err               error
	)
	if value := r.URL.Query().Get("filter"); value != "" {
		// For simplicity, only support filtering by dasplayName eq "<value>".
		parts := strings.SplitN(value, " ", 3)
		if len(parts) != 3 || parts[0] != "displayName" || parts[1] != "eq" {
			writeError(w, NewError(http.StatusBadRequest, "Unsupported filter"))
			return
		}
		nameFilter = strings.Trim(parts[2], `"`)
	}
	if value := r.URL.Query().Get("excludedAttributes"); value != "" {
		fields := strings.Split(value, ",")
		excludeMembers = slices.Contains(fields, "members")
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
	logrus.Infof("scim::ListGroups: startIndex=%d, count=%d", startIndex, count)

	groups, err := s.groupsCache.List(labels.Set{authProviderLabel: provider}.AsSelector())
	if err != nil {
		logrus.Errorf("scim::ListGroups: failed to list groups for provider %s: %s", provider, err)
		writeError(w, NewInternalError())
		return
	}

	var resources []any
	if len(groups) > 0 {
		var uniqueGroups map[string][]SCIMMember
		if !excludeMembers {
			uniqueGroups, err = s.getRancherGroups(provider)
			if err != nil {
				logrus.Errorf("scim::ListGroups: %s", err)
				writeError(w, NewInternalError())
				return
			}
		}

		for _, group := range groups {
			// Case insensitive match for displayName.
			if nameFilter != "" && !strings.EqualFold(group.DisplayName, nameFilter) {
				continue
			}

			resource := map[string]any{
				"schemas":     []string{GroupSchemaID},
				"id":          group.Name,
				"displayName": group.DisplayName,
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

			resources = append(resources, resource)
		}
	}

	response := &ListResponse{
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

func (s *SCIMServer) CreateGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::CreateGroup: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Info("scim::CreateGroup: request body:", string(bodyBytes))

	payload := SCIMGroup{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		logrus.Errorf("scim::CreateGroup: failed to unmarshal request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	list, err := s.groupsCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to list groups: %s", err)
		writeError(w, NewInternalError())
		return
	}

	for _, group := range list {
		if strings.EqualFold(group.DisplayName, payload.DisplayName) {
			writeError(w, NewError(http.StatusConflict, fmt.Sprintf("Group %s already exists", payload.ID)))
			return
		}
	}

	group, err := s.ensureRancherGroup(provider, payload)
	if err != nil {
		logrus.Errorf("scim::CreateGroup: failed to ensure rancher group: %s", err)
		writeError(w, NewInternalError())
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

func (s *SCIMServer) GetGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::GetGroup: url %s", r.URL.String())

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

	var uniqueGroups map[string][]SCIMMember
	if !excludeMembers {
		uniqueGroups, err = s.getRancherGroups(provider)
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

	writeResponse(w, resource)
}

func (s *SCIMServer) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::UpdateGroup: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}
	logrus.Info("scim::UpdateGroup: request body:", string(bodyBytes))

	payload := SCIMGroup{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		logrus.Errorf("scim::UpdateGroup: failed to unmarshal request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if id != payload.ID { // TODO: Revisit this.
		logrus.Errorf("scim::UpdateGroup: id in URL %s does not match id in body %s", id, payload.ID)
		writeError(w, NewError(http.StatusBadRequest, "Mismatched Group id"))
		return
	}

	group, err := s.ensureRancherGroup(provider, payload)
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

func (s *SCIMServer) PatchGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::PatchGroup: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	group, err := s.groupsCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Group %s not found", id)))
			return
		} else {
			logrus.Errorf("scim::PatchGroup: failed to get group: %s", err)
			writeError(w, NewInternalError())
		}
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("scim::PatchGroup: failed to read request body: %s", err)
		writeError(w, NewError(http.StatusBadRequest, "Invalid request body"))
		return
	}

	logrus.Info("scim::PatchGroup: request body:", string(bodyBytes))

	_, _, _ = provider, id, group
}

func (s *SCIMServer) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::DeleteGroup: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	group, err := s.groupsCache.Get(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Group %s not found", id)))
			return
		} else {
			logrus.Errorf("scim::DeleteGroup: failed to get group: %s", err)
			writeError(w, NewInternalError())
		}
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

func (s *SCIMServer) getRancherGroups(provider string) (map[string][]SCIMMember, error) {
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
			if !apierrors.IsNotFound(err) {
				continue
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

func (s *SCIMServer) syncGroupMembers(provider, groupName string, members []SCIMMember) error {
	uniqueGroups, err := s.getRancherGroups(provider)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	rancherMembers := uniqueGroups[groupName]
	existing := make(map[string]string, len(rancherMembers))
	for _, m := range rancherMembers {
		existing[m.Value] = m.Display
	}

	for _, member := range members {
		if _, ok := existing[member.Value]; !ok {
			// New member added.
			err := s.addGroupMember(provider, groupName, member)
			if err != nil {
				return fmt.Errorf("failed to add member %s to group %s: %s", member.Value, groupName, err)
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

func (s *SCIMServer) removeAllGroupMembers(provider, groupName string) error {
	uniqueGroups, err := s.getRancherGroups(provider)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	for _, member := range uniqueGroups[groupName] {
		err := s.removeGroupMember(provider, groupName, member.Value)
		if err != nil {
			return fmt.Errorf("failed to remove member %s from group %s: %w", member.Value, groupName, err)
		}
	}

	return nil
}

func (s *SCIMServer) ensureRancherGroup(provider string, grp SCIMGroup) (*v3.Group, error) {
	if grp.ID != "" {
		return s.groupsCache.Get(grp.ID)
	}

	// Try to find an existing group by display name.
	var group *v3.Group
	groups, err := s.groupsCache.List(labels.Set{authProviderLabel: provider}.AsSelector())
	if err != nil {
		return nil, fmt.Errorf("failed to list groups for provider %s: %w", provider, err)
	}

	for _, g := range groups {
		if g.DisplayName == grp.DisplayName {
			group = g
			break
		}
	}

	if group == nil {
		group = &v3.Group{
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

		return s.groups.Create(group)
	}

	// Check and update existing group if needed.
	shouldUpdate := false
	group = group.DeepCopy()
	if group.ExternalID != grp.ExternalID {
		group.ExternalID = grp.ExternalID
		shouldUpdate = true
	}

	if shouldUpdate {
		return s.groups.Update(group)
	}

	return group, nil
}
