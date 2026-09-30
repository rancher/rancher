package scim

import "fmt"

func userPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_user://%s", provider, id)
}

func groupPrincipalName(provider, id string) string {
	return fmt.Sprintf("%s_group://%s", provider, id)
}
