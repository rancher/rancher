package providerrefresh

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPrincipalIDForProvider(t *testing.T) {
	const testUserUsername = "tUser"
	tests := []struct {
		name               string
		userPrincipalIds   []string
		providerName       string
		desiredPrincipalId string
	}{
		{
			name:               "basic test",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name:               "no principal for provider",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "not-a-provider",
			desiredPrincipalId: "",
		},
		{
			name:               "local provider principal",
			userPrincipalIds:   []string{fmt.Sprintf("local://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: fmt.Sprintf("local://%s", testUserUsername),
		},
		{
			name:               "local provider missing principal",
			userPrincipalIds:   []string{fmt.Sprintf("local_user://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: "",
		},
		{
			name: "multiple providers, correct one (first) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
		{
			name: "multiple providers, correct one (last) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name: "multiple correct providers, first one chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("ldap_user://%s", "tUser2"),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			user := v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: testUserUsername,
				},
				PrincipalIDs: test.userPrincipalIds,
			}
			outputPrincipalID := GetPrincipalIDForProvider(test.providerName, &user)
			assert.Equal(t, test.desiredPrincipalId, outputPrincipalID, "got a different principal id than expected")
		})
	}
}
