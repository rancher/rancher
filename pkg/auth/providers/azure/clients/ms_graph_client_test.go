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
// WARNING: The test data is connected to Azure so tests can fail because the
// data changed upstream.
//
// TEST_AZURE_TENANT_ID
// TEST_AZURE_APPLICATION_ID
// TEST_AZURE_APPLICATION_SECRET
// TEST_AZURE_CHINA

var isTestAzureChina = os.Getenv("TEST_AZURE_CHINA") != ""

func TestMSGraphClient_GetUser(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping GetUser test for China region")
	}

	client := newTestClient(t)

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

	_, err = client.GetUser("testuser6@ranchertest.onmicrosoft.com")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMSGraphClient_ListUsers(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping ListUsers test for China region")
	}
	client := newTestClient(t)

	users, err := client.ListUsers("")
	if err != nil {
		t.Fatal(err)
	}

	var displayNames []string
	for _, v := range users {
		displayNames = append(displayNames, v.DisplayName)
	}

	assert.Len(t, users, 91)
}

func TestMSGraphClient_ListUsers_with_filter(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping ListUsers with filter test for China region")
	}
	client := newTestClient(t)

	users, err := client.ListUsers("startswith(userPrincipalName,'fresh')")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, users, 2)
}

func TestMSGraphClient_GetGroup(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping GetGroup test for China region")
	}
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
	if isTestAzureChina {
		t.Skip("Skipping ListGroups test for China region")
	}
	client := newTestClient(t)

	groups, err := client.ListGroups("")
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(groups), 1)
}

func TestMSGraphClient_ListGroups_with_filter(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping ListGroups with filter test for China region")
	}
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
	if isTestAzureChina {
		t.Skip("Skipping ListGroupMemberships test for China region")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser1@ranchertest.onmicrosoft.com", "")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{
		"15f6a947-9d67-4e7f-b1d0-f5f52145fed3", "5e0d1316-aa15-4c94-83e1-7db91acc7795",
		"6b2c23ed-626d-4ce4-a889-7c2043ace20e", "748274fd-3ec7-40d1-b08b-775c1a8ec1af",
		"bf881716-8d6d-456f-b234-2b143dfd5cf0"}, groups)
}

func TestMSGraphClient_ListGroupMemberships_nested_groups(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping ListGroupMemberships with nested groups test for China region")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("anunesteduser1@ranchertest.onmicrosoft.com", "")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{
		"95469c9b-7f7f-48c4-82f2-a4c1eacf454b",
		"bf6fa98e-9d06-46ed-bd62-5d8eee196265",
	}, groups)
}

func TestMSGraphClient_ListGroupMemberships_with_filter(t *testing.T) {
	if isTestAzureChina {
		t.Skip("Skipping ListGroupMemberships with filter test for China region")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser1@ranchertest.onmicrosoft.com", "")
	if err != nil {
		t.Fatal(err)
	}
	unfilteredCount := len(groups)

	groups, err = client.ListGroupMemberships("testuser1@ranchertest.onmicrosoft.com", "startswith(displayName,'test')")
	if err != nil {
		t.Fatal(err)
	}

	assert.Less(t, len(groups), unfilteredCount)
}

func TestMSGraphClient_GetUser_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China GetUser test")
	}

	secrets := newTestSecretsClient()
	client := newTestClientWithSecretsClient(t, secrets)

	user, err := client.GetUser("testuser2@rancherchina.partner.onmschina.cn")
	if err != nil {
		t.Fatal(err)
	}
	want := mgmtv3.Principal{
		PrincipalType: "user",
		Provider:      Name,
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuread_user://7372dca1-19b9-42a5-8ee5-8e936f014638",
		},
		DisplayName: "testuser2",
		LoginName:   "testuser2@rancherchina.partner.onmschina.cn",
	}
	assert.Equal(t, want, user)

	client = newTestClientWithSecretsClient(t, secrets)
	_, err = client.GetUser("testuser2@rancherchina.partner.onmschina.cn")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMSGraphClient_ListUsers_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListUsers test")
	}
	client := newTestClient(t)

	users, err := client.ListUsers("")
	if err != nil {
		t.Fatal(err)
	}

	var displayNames []string
	for _, v := range users {
		displayNames = append(displayNames, v.DisplayName)
	}
	assert.Len(t, users, 12)
}

func TestMSGraphClient_ListUsers_with_filter_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListUsers with filter test")
	}
	client := newTestClient(t)

	users, err := client.ListUsers("startswith(userPrincipalName,'test')")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, users, 2)
}

func TestMSGraphClient_GetGroup_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China GetGroup test")
	}
	client := newTestClient(t)

	group, err := client.GetGroup("3298ffd2-fc04-484d-bbd9-71c21d23dbe4")
	if err != nil {
		t.Fatal(err)
	}

	want := mgmtv3.Principal{
		PrincipalType: "group",
		Provider:      Name,
		ObjectMeta: metav1.ObjectMeta{
			Name: "azuread_group://3298ffd2-fc04-484d-bbd9-71c21d23dbe4",
		},
		DisplayName: "rancher-user",
	}
	assert.Equal(t, want, group)
}

func TestMSGraphClient_ListGroups_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListGroups test")
	}
	client := newTestClient(t)

	groups, err := client.ListGroups("")
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(groups), 1)
}

func TestMSGraphClient_ListGroups_with_filter_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListGroups with filter test")
	}
	client := newTestClient(t)

	groups, err := client.ListGroups("")
	if err != nil {
		t.Fatal(err)
	}
	unfilteredCount := len(groups)

	groups, err = client.ListGroups("startswith(displayName,'rancher')")
	if err != nil {
		t.Fatal(err)
	}

	assert.Less(t, len(groups), unfilteredCount)
}

func TestMSGraphClient_ListGroupMemberships_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListGroupMemberships test")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser1@rancherchina.partner.onmschina.cn", "")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{
		"3298ffd2-fc04-484d-bbd9-71c21d23dbe4",
	}, groups)
}

func TestMSGraphClient_ListGroupMemberships_nested_groups_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListGroupMemberships with nested groups test")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser2@rancherchina.partner.onmschina.cn", "")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []string{
		"0636b6f7-9940-412c-b4d4-bbcf3a165f85",
		"3298ffd2-fc04-484d-bbd9-71c21d23dbe4",
		"83ad5bc7-2148-43ba-8f51-c5018c5f8a8c",
	}, groups)
}

func TestMSGraphClient_ListGroupMemberships_with_filter_China(t *testing.T) {
	if !isTestAzureChina {
		t.Skip("Skipping China ListGroupMemberships with filter test")
	}
	client := newTestClient(t)

	groups, err := client.ListGroupMemberships("testuser2@rancherchina.partner.onmschina.cn", "")
	if err != nil {
		t.Fatal(err)
	}
	unfilteredCount := len(groups)

	groups, err = client.ListGroupMemberships("testuser2@rancherchina.partner.onmschina.cn", "startswith(displayName,'rancher')")
	if err != nil {
		t.Fatal(err)
	}

	assert.Less(t, len(groups), unfilteredCount)
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
	endpoint := "https://login.microsoftonline.com/"
	graphEndpoint := "https://graph.microsoft.com"

	if isTestAzureChina {
		endpoint = "https://login.partner.microsoftonline.cn/"
		graphEndpoint = "https://microsoftgraph.chinacloudapi.cn"
	}

	client, err := NewMSGraphClient(&apismgmtv3.AzureADConfig{
		Endpoint:          endpoint,
		GraphEndpoint:     graphEndpoint,
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
	return newTestSecretsClientWithMap(map[types.NamespacedName]*corev1.Secret{})
}

func newTestSecretsClientWithMap(secrets map[types.NamespacedName]*corev1.Secret) normancorev1.SecretInterface {
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
