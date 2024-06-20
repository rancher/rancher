package auth

import (
	"context"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	v3 "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

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
