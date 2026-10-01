package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserPrincipalName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "okta_user://john.doe", userPrincipalName("okta", "john.doe"))
	assert.Equal(t, "azuread_user://obj-123", userPrincipalName("azuread", "obj-123"))
}

func TestGroupPrincipalName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "okta_group://Engineering", groupPrincipalName("okta", "Engineering"))
	assert.Equal(t, "azuread_group://obj-456", groupPrincipalName("azuread", "obj-456"))
}
