package openldapauth

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

const openLdapConfigNameBase = "openLdapAuthConfig"
const ConfigurationFileKey = "authProvider"

type OpenLDAPCredentialConfig struct {
	Servers                         []string `json:"servers"`
	ServiceAccountDistinguishedName string   `json:"serviceAccountDistinguishedName"`
	ServiceAccountPassword          string   `json:"serviceAccountPassword"`
	UserSearchBase                  string   `json:"userSearchBase"`
	Port                            int      `json:"port"`
	TLS                             bool     `json:"tls"`
	// Certificate would typically be loaded from a file or another source - commenting out til openldap
	// is configured with a cert
	// Certificate                  string `json:"certificate"`
	TestUsername string `json:"testUsername"`
	TestPassword string `json:"testPassword"`
}

type Config2 struct {
	Host                            string `yaml:"host"`
	Token                           string `yaml:"token"`
	OpenLdapUser                    string `yaml:"testAndEnableUser"`
	OpenLdapUserPass                string `yaml:"testAndEnablePass"`
	Servers                         string `yaml:"servers"`
	ServiceAccountDistinguishedName string `yaml:"ServiceAccountDistinguishedName"`
	ServiceAccountPassword          string `yaml:"ServiceAccountPassword"`
	UserSearchBase                  string `yaml:"UserSearchBase"`
	Port                            string `yaml:"port"`
	TLS                             string `yaml:"TLS"`
	LoginUser                       string `yaml:"LoginUser"`
	LoginPass                       string `yaml:"LoginPass"`
}

// CreateOpenLDAPAuthConfig is a helper function that creates
// an openLDAP auth config, enables it, and returns the AuthConfig response
func CreateOpenLDAPAuthConfig(rancherClient *rancher.Client) (*management.AuthConfig, error) {

	authConfig := management.AuthConfig{
		Name:    openLdapConfigNameBase,
		Enabled: true,
	}

	resp := &management.AuthConfig{}
	err := rancherClient.Management.APIBaseClient.Ops.DoCreate(management.AuthConfigType, authConfig, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
