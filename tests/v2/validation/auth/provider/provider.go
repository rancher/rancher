package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/auth"
	v3 "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/stretchr/testify/require"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const ConfigurationFileKey = "authInput"

type User struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type AuthConfig struct {
	Group             string `json:"group,omitempty" yaml:"group,omitempty"`
	Users             []User `json:"users,omitempty" yaml:"users,omitempty"`
	NestedGroup       string `json:"nestedGroup,omitempty" yaml:"nestedGroup,omitempty"`
	NestedUsers       []User `json:"nestedUsers,omitempty" yaml:"nestedUsers,omitempty"`
	DoubleNestedGroup string `json:"doubleNestedGroup,omitempty" yaml:"doubleNestedGroup,omitempty"`
	DoubleNestedUsers []User `json:"doubleNestedUsers,omitempty" yaml:"doubleNestedUsers,omitempty"`
}

const (
	passwordSecretID                     = "cattle-global-data/openldapconfig-serviceaccountpassword"
	authProvCleanupAnnotationKey         = "management.cattle.io/auth-provider-cleanup"
	authProvCleanupAnnotationValLocked   = "rancher-locked"
	authProvCleanupAnnotationValUnlocked = "unlocked"
)

func waitUntilAnnotationIsUpdated(client *rancher.Client) (*v3.AuthConfig, error) {
	ldapConfig, err := client.Management.AuthConfig.ByID("openldap")
	if err != nil {
		return nil, err
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, 2*time.Minute, true, func(context.Context) (bool, error) {
		newLDAPConfig, err := client.Management.AuthConfig.ByID("openldap")
		if err != nil {
			return false, nil
		}

		if ldapConfig.Annotations[authProvCleanupAnnotationKey] != newLDAPConfig.Annotations[authProvCleanupAnnotationKey] {
			ldapConfig = newLDAPConfig
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return ldapConfig, err
}

var userEnabled = true

func login(client *rancher.Client, authProvider auth.Provider, user *v3.User) (*rancher.Client, error) {
	user.Enabled = &userEnabled
	return client.AsAuthUser(user, authProvider)
}

func newPrincipalID(authConfigID, principalType, name, searchBase string) string {
	return fmt.Sprintf("%v_%v://cn=%v,ou=%vs,%v", authConfigID, principalType, name, principalType, searchBase)
}

func newWithAccessMode(t *testing.T, client *rancher.Client, authConfigID, accessMode string, allowedPrincipalIDs []string) (existing, updates *v3.AuthConfig) {
	t.Helper()

	existing, err := client.Management.AuthConfig.ByID(authConfigID)
	require.NoError(t, err)

	updates = existing
	updates.AccessMode = accessMode

	if allowedPrincipalIDs != nil {
		updates.AllowedPrincipalIDs = allowedPrincipalIDs
	}

	return
}
