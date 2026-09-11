package nodedriver

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/stretchr/testify/assert"
)

func TestGetCredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		annotations      map[string]string
		expectedPublic   []string
		expectedPrivate  []string
		expectedPassword sets.Set[string]
		expectedOptional sets.Set[string]
		expectedDefaults map[string]string
	}{
		{
			name:             "nil annotations return empty metadata",
			annotations:      nil,
			expectedPublic:   nil,
			expectedPrivate:  nil,
			expectedPassword: sets.New[string](),
			expectedOptional: sets.New[string](),
			expectedDefaults: map[string]string{},
		},
		{
			name: "annotation fields are normalized into sorted sets",
			annotations: map[string]string{
				"publicCredentialFields":   "username,accessKey,,endpoint,accessKey",
				"privateCredentialFields":  ",password,secretKey,password",
				"passwordFields":           "password,,token,password",
				"optionalCredentialFields": "endpoint,,password,endpoint",
				"defaults":                 "endpoint:example.com,region:us-west-2",
			},
			expectedPublic:   []string{"accessKey", "endpoint", "username"},
			expectedPrivate:  []string{"password", "secretKey"},
			expectedPassword: sets.New[string]("password", "token"),
			expectedOptional: sets.New[string]("endpoint", "password"),
			expectedDefaults: map[string]string{
				"endpoint": "example.com",
				"region":   "us-west-2",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getCredFields(tt.annotations)

			assert.Equal(t, tt.expectedPublic, result.publicList())
			assert.Equal(t, tt.expectedPrivate, result.privateList())
			assert.Equal(t, tt.expectedPassword, result.password)
			assert.Equal(t, tt.expectedOptional, result.optional)
			assert.Equal(t, tt.expectedDefaults, result.defaults)
			assert.False(t, result.public.Has(""))
			assert.False(t, result.private.Has(""))
			assert.False(t, result.password.Has(""))
			assert.False(t, result.optional.Has(""))
		})
	}
}
