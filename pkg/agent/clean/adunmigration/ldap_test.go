package adunmigration

import (
	"testing"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestEscapeUUID(t *testing.T) {
	t.Parallel()
	// Note: Because Rancher wrote these strings, the input format we care about is somewhat
	// constrained, and we're not spending too much effort dealing with edge cases.
	guidString := "953d82a03d47a5498330293e386dfce1"
	wantEscapedString := "\\95\\3d\\82\\a0\\3d\\47\\a5\\49\\83\\30\\29\\3e\\38\\6d\\fc\\e1"
	result := escapeUUID(guidString)
	assert.Equal(t, wantEscapedString, result)
}

func TestIsGUID(t *testing.T) {
	// Note: because logrus state is global, we cannot use t.Parallel here
	hook := logrusTest.NewGlobal()

	tests := []struct {
		name            string
		principalId     string
		wantResult      bool
		wantLoggedError bool
	}{
		{
			name:            "Local User",
			principalId:     "local://u-fydcaomakf",
			wantResult:      false,
			wantLoggedError: false,
		},
		{
			name:            "AD user, DN based",
			principalId:     "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
			wantResult:      false,
			wantLoggedError: false,
		},
		{
			name:            "AD user, GUID based",
			principalId:     "activedirectory_user://953d82a03d47a5498330293e386dfce1",
			wantResult:      true,
			wantLoggedError: false,
		},
		{
			name:            "Invalid principal",
			principalId:     "fail-on-purpose",
			wantResult:      false,
			wantLoggedError: true,
		},
		{
			name:            "Empty String",
			principalId:     "",
			wantResult:      false,
			wantLoggedError: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result := isGUID(test.principalId)
			if result != test.wantResult {
				t.Errorf("expected isGUID to be %v, but got %v", test.wantResult, result)
			}
			// This particular function treats invalid principal IDs as a soft error: an invalid ID is by
			// definition not a valid GUID. We generate a log if this happens, so let's make sure we got
			// that log:
			if test.wantLoggedError {
				assert.GreaterOrEqual(t, len(hook.Entries), 1)
				assert.Equal(t, hook.LastEntry().Level, logrus.ErrorLevel)
			}
			hook.Reset()
		})
	}
}
