package openldapauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpenLdapTest struct {
	suite.Suite
	testUser *management.User
	client   *rancher.Client
	project  *management.Project
	session  *session.Session
}

type APIResponse struct {
	StatusCode int
	Body       string
}

type LoginRequest struct {
	Description  string `json:"description"`
	ResponseType string `json:"responseType"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

type LdapConfig struct {
	Actions                         map[string]string `json:"actions"`
	Annotations                     map[string]string `json:"annotations"`
	BaseType                        string            `json:"baseType"`
	ConnectionTimeout               int               `json:"connectionTimeout"`
	CreatorId                       interface{}       `json:"creatorId"`
	Enabled                         bool              `json:"enabled"`
	GroupDNAttribute                string            `json:"groupDNAttribute"`
	GroupMemberMappingAttribute     string            `json:"groupMemberMappingAttribute"`
	GroupMemberUserAttribute        string            `json:"groupMemberUserAttribute"`
	GroupNameAttribute              string            `json:"groupNameAttribute"`
	GroupObjectClass                string            `json:"groupObjectClass"`
	GroupSearchAttribute            string            `json:"groupSearchAttribute"`
	ID                              string            `json:"id"`
	Labels                          map[string]string `json:"labels"`
	Links                           map[string]string `json:"links"`
	Name                            string            `json:"name"`
	NestedGroupMembershipEnabled    bool              `json:"nestedGroupMembershipEnabled"`
	Port                            int               `json:"port"`
	Starttls                        bool              `json:"starttls"`
	Tls                             bool              `json:"tls"`
	Type                            string            `json:"type"`
	UserDisabledBitMask             int               `json:"userDisabledBitMask"`
	UserLoginAttribute              string            `json:"userLoginAttribute"`
	UserMemberAttribute             string            `json:"userMemberAttribute"`
	UserNameAttribute               string            `json:"userNameAttribute"`
	UserObjectClass                 string            `json:"userObjectClass"`
	UserSearchAttribute             string            `json:"userSearchAttribute"`
	UUID                            string            `json:"uuid"`
	Clone                           bool              `json:"__clone"`
	Servers                         []string          `json:"servers"`
	AccessMode                      string            `json:"accessMode"`
	DisabledStatusBitmask           int               `json:"disabledStatusBitmask"`
	ServiceAccountDistinguishedName string            `json:"serviceAccountDistinguishedName"`
	ServiceAccountPassword          string            `json:"serviceAccountPassword"`
	UserSearchBase                  string            `json:"userSearchBase"`
	GroupSearchBase                 string            `json:"groupSearchBase"`
}

type Payload struct {
	Enabled    bool       `json:"enabled"`
	LdapConfig LdapConfig `json:"ldapConfig"`
	Username   string     `json:"username"`
	Password   string     `json:"password"`
}

func (d *OpenLdapTest) SetupSuite() {

	testSession := session.NewSession()
	d.session = testSession
	logrus.Info("setup suite")
	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(d.T(), err)

	d.project = testProject
}

func (d *OpenLdapTest) TearDownSuite() {
	d.session.Cleanup()
}

func (d *OpenLdapTest) TestOpenLdapAPI() {
	testSession := session.NewSession()
	defer testSession.Cleanup()

	upgradeConfig := new(Config2)
	config.LoadConfig(ConfigurationFileKey, upgradeConfig)

	fmt.Print(d.client.RancherConfig.ClusterName)
	fmt.Print(d.client.RancherConfig.Host)

	fmt.Print(upgradeConfig.Host)
	fmt.Print(upgradeConfig.LoginUser)
	url := "https://ron1016.qa.rancher.space/v3/openLdapConfigs/openldap?action=testAndApply"
	token := "token-vs9qg:pg8g9jcksx4pw6h7xh7448c6m94h8nmc45ln88q6wr9gfk57qvftp5"

	resp, err := d.EnableOpenLdap(url, token)
	require.NoError(d.T(), err)

	validStatus := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusInternalServerError
	errorMessage := fmt.Sprintf("Expected status code 200, 201, or 500, but got %d", resp.StatusCode)

	if resp.StatusCode == http.StatusInternalServerError {
		errorMessage = "openldap user can't login when openldap disabled"
	}

	require.True(d.T(), validStatus, errorMessage)

}

func (d *OpenLdapTest) EnableOpenLdap(url string, token string) (*APIResponse, error) {
	payloadInstance := Payload{
		Enabled: true,
		LdapConfig: LdapConfig{
			Actions: map[string]string{
				"testAndApply": "https://ron1016.qa.rancher.space/v3/openLdapConfigs/openldap?action=testAndApply",
			},
			Annotations: map[string]string{
				"management.cattle.io/auth-provider-cleanup": "rancher-locked",
			},
			BaseType:                    "authConfig",
			ConnectionTimeout:           5000,
			CreatorId:                   nil,
			Enabled:                     true,
			GroupDNAttribute:            "entryDN",
			GroupMemberMappingAttribute: "member",
			GroupMemberUserAttribute:    "entryDN",
			GroupNameAttribute:          "cn",
			GroupObjectClass:            "groupOfNames",
			GroupSearchAttribute:        "cn",
			ID:                          "openldap",
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
			Links: map[string]string{
				"self":   "https://ron1016.qa.rancher.space/v3/openLdapConfigs/openldap",
				"update": "https://ron1016.qa.rancher.space/v3/openLdapConfigs/openldap",
			},
			Name:                            "openldap",
			NestedGroupMembershipEnabled:    false,
			Port:                            389,
			Starttls:                        false,
			Tls:                             false,
			Type:                            "openLdapConfig",
			UserDisabledBitMask:             0,
			UserLoginAttribute:              "uid",
			UserMemberAttribute:             "memberOf",
			UserNameAttribute:               "cn",
			UserObjectClass:                 "inetOrgPerson",
			UserSearchAttribute:             "uid|sn|givenName",
			UUID:                            "c30859dd-f103-446b-ad81-9633f2da0438",
			Clone:                           true,
			Servers:                         []string{"openldapqa.qa.rancher.space"},
			AccessMode:                      "unrestricted",
			DisabledStatusBitmask:           0,
			ServiceAccountDistinguishedName: "cn=admin,dc=qa,dc=rancher,dc=space",
			ServiceAccountPassword:          "cattle@123",
			UserSearchBase:                  "dc=qa,dc=rancher,dc=space",
			GroupSearchBase:                 "",
		},
		Username: "testuser1",
		Password: "Tacos86!",
	}

	body, err := json.MarshalIndent(payloadInstance, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling:", err)
		return nil, err
	}

	fmt.Println(string(body))

	resp, err := SendAPICall(url, token, body)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Println("API call was successful!")
	} else {
		fmt.Printf("API call failed with status code: %d\n", resp.StatusCode)
		fmt.Println("Response Body:", resp.Body)
	}
	return resp, nil

}
func (d *OpenLdapTest) LoginOpenLDAP(host string, token string, body []byte) (*APIResponse, error) {
	url := "https://ron1016.qa.rancher.space/v3-public/openLdapProviders/openldap?action=login"
	token = "token-vs9qg:pg8g9jcksx4pw6h7xh7448c6m94h8nmc45ln88q6wr9gfk57qvftp5"

	body = []byte(`{
		"description": "postman",
		"responseType": "token",
		"username": "testuser2",
		"password": "Tacos86!"
	}`)

	return SendAPICall(url, token, body)
}

func (d *OpenLdapTest) DisableOpenLDAP(url string, token string) (*APIResponse, error) {
	url = "https://ron1016.qa.rancher.space/v3/openLdapConfigs/openldap?action=disable"
	body := []byte(`{"action": "disable"}`)

	resp, err := SendAPICall(url, token, body)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Println("API call was successful!")
	} else {
		fmt.Printf("API call failed with status code: %d\n", resp.StatusCode)
		fmt.Println("Response Body:", resp.Body)
	}
	return resp, nil
}

func SendAPICall(url, token string, body []byte) (*APIResponse, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	fmt.Println(url, token, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}, nil
}

func TestOpenLdapTestSuite(t *testing.T) {
	suite.Run(t, new(OpenLdapTest))
}
