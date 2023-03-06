package keycloakoidc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

// account defines properties an account in keycloak has
type account struct {
	ID            string `json:"id,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"emailVerified,omitempty"`
	Username      string `json:"username,omitempty"`
	Enabled       bool   `json:"enabled,omitempty"`
	Name          string `json:"firstName,omitempty"`
	LastName      string `json:"lastName,omitempty"`
	Type          string
}

// Group defines properties a group in keycloak has
type Group struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name,omitempty"`
	Subgroups []Group `json:"subGroups,omitempty"`
}

// KeyCloakClient implements a httpclient for keycloak
type KeyCloakClient struct {
	httpClient *http.Client
}

func (k *KeyCloakClient) searchPrincipals(searchTerm, principalType string, config *v32.OIDCConfig) ([]account, error) {
	var accounts []account
	sURL, err := getSearchURL(config.Issuer)
	if err != nil {
		return accounts, err
	}
	if principalType == "" || principalType == UserType {
		var userAccounts []account
		searchURL := fmt.Sprintf("%s/%ss?search=%s", sURL, UserType, searchTerm)
		search := URLEncoded(searchURL)

		b, err := k.getFromKeyCloak(search)
		if err != nil {
			logrus.Errorf("[keycloak oidc] searchPrincipals: GET request failed. url: %s, err: %s", search, err)
			return accounts, err
		}
		if err := json.Unmarshal(b, &userAccounts); err != nil {
			logrus.Errorf("[keycloak oidc] searchPrincipals: received error unmarshalling search results, err: %v", err)
			return accounts, err
		}
		for _, u := range userAccounts {
			u.Type = UserType
			accounts = append(accounts, u)
		}
	}
	//checking the GroupSearchEnabled flag to ensure group principals are not returned if group mappers
	//are not enabled. If group mappers are not enabled, it doesn't make sense to return groups as
	//principals that could be authorized.
	if (principalType == "" || principalType == GroupType) && *config.GroupSearchEnabled == true {
		groupAccounts, err := k.groupSearch(searchTerm, sURL)
		if err != nil {
			return accounts, err
		}
		accounts = append(accounts, groupAccounts...)
	}
	return accounts, nil
}

func (k *KeyCloakClient) groupSearch(searchTerm string, sURL string) ([]account, error) {
	var groups []Group
	var accounts []account

	searchURL := fmt.Sprintf("%s/%ss?search=%s", sURL, GroupType, searchTerm)
	search := URLEncoded(searchURL)

	b, err := k.getFromKeyCloak(search)
	if err != nil {
		logrus.Errorf("[keycloak oidc] groupSearch: GET request failed. url: %s, err: %s", search, err)
		return accounts, err
	}
	if err = json.Unmarshal(b, &groups); err != nil {
		logrus.Errorf("[keycloak oidc] groupSearch: received error unmarshalling search results, err: %v", err)
		return accounts, err
	}
	for _, g := range groups {
		accounts = append(accounts, account{ID: g.ID, Name: g.Name, Type: GroupType})
		subGroups := getSubGroups(g)
		for _, sg := range subGroups {
			accounts = append(accounts, account{ID: sg.ID, Name: sg.Name, Type: GroupType})
		}
	}
	return accounts, nil
}

func filterByGroupName(name string, accounts []account) account {
	for _, group := range accounts {
		if group.Name == name {
			return group
		}
	}
	return account{}
}

func getSubGroups(group Group) []Group {
	var groups []Group
	if len(group.Subgroups) > 0 {
		for i, sub := range group.Subgroups {
			// setting an upper limit for how many subgroups we will loop through
			// this value was chosen at random so can be changed if needed
			if i < 100 {
				groups = append(groups, sub)
				groups = append(groups, getSubGroups(sub)...)
			}
		}
	}
	return groups
}

func (k *KeyCloakClient) getFromKeyCloakByID(principalID, principalType string, config *v32.OIDCConfig) (account, error) {
	var searchResult account

	if principalID == "" {
		return searchResult, errors.Errorf("invalid id %v", principalID)
	}
	sURL, err := getSearchURL(config.Issuer)
	if err != nil {
		return searchResult, nil
	}
	// this will use the keycloak search endpoint with an id
	if principalType == UserType {
		searchURL := fmt.Sprintf("%s/%ss/%s", sURL, principalType, principalID)
		search := URLEncoded(searchURL)
		b, err := k.getFromKeyCloak(search)
		if err != nil {
			logrus.Errorf("[keycloak oidc] getFromKeyCloakByID: GET request failed. url: %s, err: %s", search, err)
			return searchResult, err
		}
		if err := json.Unmarshal(b, &searchResult); err != nil {
			logrus.Errorf("[keycloak oidc] getFromKeyCloakByID: received error unmarshalling search results, err: %v", err)
			return searchResult, err
		}
	} else {
		// when getting a users groups, we are only able to get the group name in some instances but
		// you must have a group's id to utilize the keycloak by id search endpoint.
		// to search by name, this uses the generic group search endpoint and then filters the result to the
		// group name. group names are unique in keycloak.
		accounts, err := k.groupSearch(principalID, sURL)
		if err != nil {
			return searchResult, err
		}
		searchResult = filterByGroupName(principalID, accounts)
		return searchResult, nil
	}
	return searchResult, nil
}

func getSearchURL(issuer string) (string, error) {
	iss := strings.SplitAfter(issuer, "/auth/") // keycloak < 19 has auth prefix
	if len(iss) == 2 {
		return fmt.Sprintf("%sadmin/%s", iss[0], iss[1]), nil
	}
	iss = strings.SplitN(issuer, "/realms/", 2) // keycloak >= 19 doesn't have auth prefix
	if len(iss) == 2 {
		return fmt.Sprintf("%s/admin/realms/%s", iss[0], iss[1]), nil
	}
	return "", fmt.Errorf("can't parse issuer url")
}

// URLEncoded encodes the string
func URLEncoded(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		logrus.Errorf("[keycloak oidc] URLEncoded: Error encoding the url: %s, error: %v", str, err)
		return str
	}
	return u.String()
}

func (k *KeyCloakClient) getFromKeyCloak(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := k.httpClient.Do(req)
	if err != nil {
		logrus.Errorf("[keycloak oidc] getFromKeyCloak: received error from keycloak: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return b, err
	}
	switch resp.StatusCode {
	case 200:
	case 201:
	case 403:
		return b, httperror.NewAPIError(httperror.PermissionDenied, "access denied")
	case 401:
		return b, httperror.NewAPIError(httperror.Unauthorized, "invalid token")
	default:
		return b, err
	}
	return b, nil
}
