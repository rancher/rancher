package adunmigration

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

func TestIsAdUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		user       v3.User
		wantResult bool
	}{
		{
			name:       "Local user",
			user:       v3.User{PrincipalIDs: []string{"local://u-fydcaomakf"}},
			wantResult: false,
		},
		{
			name: "AD user, DN based",
			user: v3.User{PrincipalIDs: []string{
				"local://u-fydcaomakf",
				"activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space"}},
			wantResult: true,
		},
		{
			name: "AD user, GUID based",
			user: v3.User{PrincipalIDs: []string{
				"local://u-fydcaomakf",
				"activedirectory_user://953d82a03d47a5498330293e386dfce1"}},
			wantResult: true,
		},
		{
			name: "Non-local, non-AD user",
			user: v3.User{PrincipalIDs: []string{
				"local://u-fydcaomakf",
				"okta_user://test.user@example.com"}},
			wantResult: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := isAdUser(&test.user)
			if result != test.wantResult {
				t.Errorf("expected isAdUser to be %v, but got %v", test.wantResult, result)
			}
		})
	}
}

func TestGetExternalId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		principalId string
		wantResult  string
		wantError   bool
	}{
		{
			name:        "Local User",
			principalId: "local://u-fydcaomakf",
			wantResult:  "u-fydcaomakf",
			wantError:   false,
		},
		{
			name:        "AD user, DN based",
			principalId: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
			wantResult:  "CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
			wantError:   false,
		},
		{
			name:        "AD user, GUID based",
			principalId: "activedirectory_user://953d82a03d47a5498330293e386dfce1",
			wantResult:  "953d82a03d47a5498330293e386dfce1",
			wantError:   false,
		},
		{
			name:        "Invalid principal",
			principalId: "fail-on-purpose",
			wantResult:  "",
			wantError:   true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := getExternalID(test.principalId)
			if test.wantError && err == nil {
				t.Errorf("expected error for invalid principalId %v", test.principalId)
			}
			if result != test.wantResult {
				t.Errorf("expected getExternalID to be %v, but got %v", test.wantResult, result)
			}
		})
	}
}

func TestGetScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		principalId string
		wantResult  string
		wantError   bool
	}{
		{
			name:        "Local User",
			principalId: "local://u-fydcaomakf",
			wantResult:  "local",
			wantError:   false,
		},
		{
			name:        "AD user, DN based",
			principalId: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
			wantResult:  "activedirectory_user",
			wantError:   false,
		},
		{
			name:        "AD user, GUID based",
			principalId: "activedirectory_user://953d82a03d47a5498330293e386dfce1",
			wantResult:  "activedirectory_user",
			wantError:   false,
		},
		{
			name:        "Invalid principal",
			principalId: "fail-on-purpose",
			wantResult:  "",
			wantError:   true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := getScope(test.principalId)
			if test.wantError && err == nil {
				t.Errorf("expected error for invalid principalId %v", test.principalId)
			}
			if result != test.wantResult {
				t.Errorf("expected getExternalID to be %v, but got %v", test.wantResult, result)
			}
		})
	}
}
