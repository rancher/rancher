package clients

import (
	"os"
	"testing"

	apismgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	normancorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/apis/core"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: This will require the following environment variables setup
//
// TEST_AZURE_TENANT_ID
// TEST_AZURE_APPLICATION_ID
// TEST_AZURE_APPLICATION_SECRET

func TestMSGraphClient_GetUser(t *testing.T) {
	secrets := newTestSecretsClient()
	client := newTestClientWithSecretsClient(t, secrets)

	user, err := client.GetUser("testuser6@ranchertest.onmicrosoft.com")
	if err != nil {
		t.Fatal(err)
	}

	want := mgmtv3.Principal{
		PrincipalType: "user",
		Provider:      Name,
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuread_user://b2511543-7052-431b-a97d-02e1e9cae337",
		},
		DisplayName: "testuser6",
		LoginName:   "testuser6@ranchertest.onmicrosoft.com",
	}
	assert.Equal(t, want, user)

	client = newTestClientWithSecretsClient(t, secrets)
	_, err = client.GetUser("testuser6@ranchertest.onmicrosoft.com")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMSGraphClient_ListUsers(t *testing.T) {
	client := newTestClient(t)

	users, err := client.ListUsers("")
	if err != nil {
		t.Fatal(err)
	}

	var displayNames []string
	for _, v := range users {
		displayNames = append(displayNames, v.DisplayName)
	}

	assert.Len(t, users, 38)
}

func TestMSGraphClient_ListUsers_with_filter(t *testing.T) {
	client := newTestClient(t)

	users, err := client.ListUsers("startswith(userPrincipalName,'fresh')")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, users, 2)
}

func TestMSGraphClient_GetGroup(t *testing.T) {
	client := newTestClient(t)

	group, err := client.GetGroup("00d7a0e6-e0b1-44be-8577-0fb76b13e853")
	if err != nil {
		t.Fatal(err)
	}

	want := mgmtv3.Principal{
		PrincipalType: "group",
		Provider:      Name,
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuread_group://00d7a0e6-e0b1-44be-8577-0fb76b13e853",
		},
		DisplayName: "lotsofgroups728",
	}
	assert.Equal(t, want, group)
}

func TestMSGraphClient_ListGroups(t *testing.T) {
	client := newTestClient(t)

	groups, err := client.ListGroups("")
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(groups), 1)
}

func TestMSGraphClient_ListGroups_with_filter(t *testing.T) {
	client := newTestClient(t)

	groups, err := client.ListGroups("")
	if err != nil {
		t.Fatal(err)
	}
	unfilteredCount := len(groups)

	groups, err = client.ListGroups("startswith(displayName,'test')")
	if err != nil {
		t.Fatal(err)
	}

	assert.Less(t, len(groups), unfilteredCount)
}

func TestMSGraphClient_ListGroupMemberships(t *testing.T) {
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser1@ranchertest.onmicrosoft.com")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"15f6a947-9d67-4e7f-b1d0-f5f52145fed3", "bf881716-8d6d-456f-b234-2b143dfd5cf0"}, groups)
}

func newTestClient(t *testing.T) *AzureMSGraphClient {
	t.Helper()
	return newTestClientWithSecretsClient(t, newTestSecretsClient())
}

func newTestClientWithSecretsClient(t *testing.T, secrets normancorev1.SecretInterface) *AzureMSGraphClient {
	t.Helper()
	tenantID, applicationID, applicationSecret := os.Getenv("TEST_AZURE_TENANT_ID"), os.Getenv("TEST_AZURE_APPLICATION_ID"), os.Getenv("TEST_AZURE_APPLICATION_SECRET")

	if tenantID == "" || applicationID == "" || applicationSecret == "" {
		t.Skip("Skipping MSGraph Client Tests for Azure because missing environment variables, TEST_AZURE_TENANT_ID, TEST_AZURE_APPLICATION_ID and TEST_AZURE_APPLICATION_SECRET must be set")
	}

	client, err := NewMSGraphClient(&apismgmtv3.AzureADConfig{
		Endpoint:          "https://login.microsoftonline.com/",
		GraphEndpoint:     "https://graph.microsoft.com",
		TenantID:          tenantID,
		ApplicationID:     applicationID,
		ApplicationSecret: applicationSecret,
	}, secrets)

	if err != nil {
		t.Fatalf("creating MSGraphClient: %s", err)
	}

	return client
}

func newTestSecretsClient() normancorev1.SecretInterface {
	secrets := map[types.NamespacedName]*corev1.Secret{}

	sm := &fakes.SecretInterfaceMock{
		GetNamespacedFunc: func(namespace, name string, opts metav1.GetOptions) (*corev1.Secret, error) {
			namespacedName := types.NamespacedName{Namespace: namespace, Name: name}
			if s, ok := secrets[namespacedName]; ok {
				return s, nil
			}

			return nil, apierrors.NewNotFound(core.Resource("Secret"), namespacedName.String())
		},

		CreateFunc: func(in1 *corev1.Secret) (*corev1.Secret, error) {
			if in1.StringData != nil {
				if in1.Data == nil {
					in1.Data = map[string][]byte{}
				}
				for k, v := range in1.StringData {
					in1.Data[k] = []byte(v)
				}
				in1.StringData = nil
			}
			secrets[client.ObjectKeyFromObject(in1)] = in1
			return in1, nil
		},

		UpdateFunc: func(in1 *corev1.Secret) (*corev1.Secret, error) {
			if in1.StringData != nil {
				if in1.Data == nil {
					in1.Data = map[string][]byte{}
				}
				for k, v := range in1.StringData {
					in1.Data[k] = []byte(v)
				}
				in1.StringData = nil
			}
			secrets[client.ObjectKeyFromObject(in1)] = in1
			return in1, nil
		},

		ControllerFunc: func() normancorev1.SecretController {
			return &fakes.SecretControllerMock{
				ListerFunc: func() normancorev1.SecretLister {
					return &fakes.SecretListerMock{
						GetFunc: func(namespace, name string) (*corev1.Secret, error) {
							namespacedName := types.NamespacedName{Namespace: namespace, Name: name}
							if s, ok := secrets[namespacedName]; ok {
								return s, nil
							}

							return nil, apierrors.NewNotFound(core.Resource("Secret"), namespacedName.String())
						},
					}
				},
			}
		},
	}

	return sm
}
