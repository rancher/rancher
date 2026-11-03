# `auth_configs_test.go` Summary

Verifies that the expected set of authentication configuration types are available and properly secured via the API.

## `TestAuthConfigsExistAndCannotBeDeleted`
**Act:** Lists all auth configs from the API and attempts to delete each one.

**Assert:**
- Checks all 18 expected auth config types (activeDirectory, adfs, azureAD, cognito, freeIpa, genericOIDC, genericSAML, githubApp, github, googleOauth, keycloak, keycloakOIDC, local, oidc, okta, openLdap, ping, shibboleth) are present in the response.
- Checks attempting to delete any auth config returns 405 Method Not Allowed.

## `TestAuthConfigActions`
**Act:** Lists all auth configs from the API.

**Assert:**
- Checks 10 configs (activeDirectory, azureAD, cognito, freeIpa, genericOIDC, githubApp, github, googleOauth, oidc, openLdap) have the testAndApply action.
- Checks 7 configs (azureAD, cognito, genericOIDC, githubApp, github, googleOauth, oidc) have the configureTest action.
- Checks 6 configs (adfs, genericSAML, keycloak, okta, ping, shibboleth) have the testAndEnable action.

## `TestAuthConfigSecrets`
**Act:** Enables the ping auth config and sets its spKey field.

**Assert:**
- Checks a "pingconfig-spkey" secret is created in the cattle-global-data namespace.
- Checks secrets for other unconfigured SAML providers ("adfsconfig-spkey", "oktaconfig-spkey", "keycloakconfig-spkey") are NOT created.
