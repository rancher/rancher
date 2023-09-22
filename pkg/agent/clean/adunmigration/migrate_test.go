package adunmigration

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkUnitContainsName(t *testing.T) {
	t.Parallel()

	originalUserName := "u-fydcaomakf"
	duplicateUserName := "u-quuknbiweg"

	workunit := migrateUserWorkUnit{
		originalUser: &v3.User{ObjectMeta: metav1.ObjectMeta{Name: originalUserName}},
		duplicateUsers: []*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: duplicateUserName}},
		},
	}

	tests := []struct {
		name       string
		query      string
		wantResult bool
	}{
		{
			name:       "No match returns false",
			query:      "u-nonexistent",
			wantResult: false,
		},
		{
			name:       "Original user match returns true",
			query:      originalUserName,
			wantResult: true,
		},
		{
			name:       "Duplicate user match returns true",
			query:      duplicateUserName,
			wantResult: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := workUnitContainsName(&workunit, test.query)
			assert.Equal(t, test.wantResult, result)
		})
	}
}
