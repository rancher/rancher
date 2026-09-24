# `auth_configs_test.go` Summary

Verifies that the expected set of authentication configuration types are available and properly secured via the API.

## `TestAuthConfigsExistAndCannotBeDeleted`
Lists all auth configs from the API and verifies that all 18 expected config types (activeDirectory, adfs, azureAD, cognito, freeIpa, genericOIDC, genericSAML, githubApp, github, googleOauth, keycloak, keycloakOIDC, local, oidc, okta, openLdap, ping, shibboleth) are present, then attempts to delete each one.
- Checks all 18 expected auth config types are returned in the response.
- Checks attempting to delete any auth config returns 405 Method Not Allowed.

## `TestAuthConfigActions`
Lists all auth configs and verifies each type exposes the appropriate action set.
- Checks 10 configs (activeDirectory, azureAD, cognito, freeIpa, genericOIDC, githubApp, github, googleOauth, oidc, openLdap) have the testAndApply action.
- Checks 7 configs (azureAD, cognito, genericOIDC, githubApp, github, googleOauth, oidc) have the configureTest action.
- Checks 6 configs (adfs, genericSAML, keycloak, okta, ping, shibboleth) have the testAndEnable action.

## `TestAuthConfigSecrets`
Enables the ping auth config, sets the spKey field, and waits for the controller to create the corresponding secret in cattle-global-data namespace.
- Checks the pingconfig-spkey secret is created in the cattle-global-data namespace.
- Checks secrets for other unconfigured SAML providers (adfsconfig-spkey, oktaconfig-spkey, keycloakconfig-spkey) are NOT created.
