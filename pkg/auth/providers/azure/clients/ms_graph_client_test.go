package clients

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

func TestAzureClient_connection_failures(t *testing.T) {
	// This creates a listener on a random available port.
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	config := &mgmtv3.AzureADConfig{
		GraphEndpoint: fmt.Sprintf("https://localhost:%d/", port),
		TenantID:      "test-tenant",
	}

	connectionTests := []struct {
		name string
		call func(client AzureClient) error
	}{
		{
			name: "GetUser()",
			call: func(client AzureClient) error {
				_, err := client.GetUser("test-user-id")
				return err
			},
		},
		{
			name: "ListUsers()",
			call: func(client AzureClient) error {
				_, err := client.ListUsers("LastName eq 'Smith'")
				return err
			},
		},
		{
			name: "GetGroup()",
			call: func(client AzureClient) error {
				_, err := client.GetGroup("testing-group")
				return err
			},
		},
		{
			name: "ListGroups()",
			call: func(client AzureClient) error {
				_, err := client.ListGroups("mailEnabled eq true")
				return err
			},
		},
		{
			name: "ListGroupMemberships()",
			call: func(client AzureClient) error {
				_, err := client.ListGroupMemberships("test-user-id")
				return err
			},
		},
	}

	for _, tt := range connectionTests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMSGraphClient(config, confidential.AuthResult{
				AccessToken: "test-token",
			})
			client.userClient.BaseClient.RetryableClient.RetryMax = 1
			client.groupClient.BaseClient.RetryableClient.RetryMax = 1

			err := tt.call(client)

			if err == nil {
				t.Error("expected to get an error, got nil")
			}

			if msg := err.Error(); !strings.Contains(msg, "connect: connection refused") {
				t.Errorf("got %s, want message with 'connection refused'", msg)
			}
		})
	}
}

func TestAzureClient_invalid_responses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths := []string{
			"/v1.0/test-tenant/users/test-user-id",
			"/v1.0/test-tenant/users",
			"/v1.0/test-tenant/groups/testing-group",
			"/v1.0/test-tenant/groups",
			"/v1.0/test-tenant/users/test-user-id/transitiveMemberOf",
		}
		if slices.Contains(paths, r.URL.Path) {
			fmt.Fprintln(w, `{ "`+strings.Repeat("a", 513)+`" 1 }`)
			return
		}
		http.Error(w, fmt.Sprintf("didn't match: %s", r.URL.Path), http.StatusNotFound)
	}))
	defer ts.Close()

	config := &mgmtv3.AzureADConfig{
		GraphEndpoint: ts.URL,
		TenantID:      "test-tenant",
	}

	connectionTests := []struct {
		name string
		call func(client AzureClient) error
	}{
		{
			name: "GetUser()",
			call: func(client AzureClient) error {
				_, err := client.GetUser("test-user-id")
				return err
			},
		},
		{
			name: "ListUsers()",
			call: func(client AzureClient) error {
				_, err := client.ListUsers("LastName eq 'Smith'")
				return err
			},
		},
		{
			name: "GetGroup()",
			call: func(client AzureClient) error {
				_, err := client.GetGroup("testing-group")
				return err
			},
		},
		{
			name: "ListGroups()",
			call: func(client AzureClient) error {
				_, err := client.ListGroups("mailEnabled eq true")
				return err
			},
		},
		{
			name: "ListGroupMemberships()",
			call: func(client AzureClient) error {
				_, err := client.ListGroupMemberships("test-user-id")
				return err
			},
		},
	}

	for _, tt := range connectionTests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMSGraphClient(config, confidential.AuthResult{
				AccessToken: "test-token",
			})
			client.userClient.BaseClient.RetryableClient.RetryMax = 1
			client.groupClient.BaseClient.RetryableClient.RetryMax = 1

			err := tt.call(client)

			if err == nil {
				t.Error("expected to get an error, got nil")
			}

			if msg := err.Error(); !strings.Contains(msg, "invalid character") {
				t.Errorf("got %s, want message with 'invalid character'", msg)
			}
		})
	}
}
