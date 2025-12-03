# Authentication Providers in Rancher

Rancher supports various authentication providers for user authentication and authorization. This document provides information about the supported authentication providers and their API availability.

## Supported Authentication Providers

The following authentication providers are fully implemented in Rancher and available via the Rancher Management API:

### OAuth/OIDC Providers
- **GitHub** (`github`) - OAuth authentication using GitHub
- **GitHub App** (`githubapp`) - OAuth authentication using GitHub Apps
- **Google OAuth** (`googleoauth`) - OAuth authentication using Google
- **Azure AD** (`azuread`) - OAuth authentication using Microsoft Azure Active Directory
- **Generic OIDC** (`genericoidc`) - Generic OpenID Connect provider support
- **KeyCloak OIDC** (`keycloakoidc`) - OpenID Connect for KeyCloak
- **OIDC** (`oidc`) - Standard OpenID Connect provider
- **Amazon Cognito** (`cognito`) - AWS Cognito User Pools authentication

### LDAP Providers
- **Active Directory** (`activedirectory`) - Microsoft Active Directory
- **OpenLDAP** (`openldap`) - OpenLDAP authentication
- **FreeIPA** (`freeipa`) - FreeIPA authentication

### SAML Providers
- **Ping Identity** (`ping`) - SAML authentication using Ping Identity
- **ADFS** (`adfs`) - SAML authentication using Active Directory Federation Services
- **KeyCloak SAML** (`keycloak`) - SAML authentication using KeyCloak
- **Okta SAML** (`okta`) - SAML authentication using Okta
- **Shibboleth** (`shibboleth`) - SAML authentication using Shibboleth

### Local Provider
- **Local** (`local`) - Rancher's built-in local authentication

## Amazon Cognito Provider

Amazon Cognito is fully supported as an OIDC authentication provider in Rancher. The Cognito provider (`cognito`) implements the OpenID Connect protocol and supports:

- User authentication via AWS Cognito User Pools
- Group membership from Cognito user groups
- Single Sign-On (SSO)
- Single Logout (SLO) when configured with the appropriate endpoints

### API Configuration

The Cognito authentication provider can be configured via the Rancher Management API using the `CognitoConfig` resource. The configuration includes:

- `clientId` - The Cognito App Client ID
- `clientSecret` - The Cognito App Client Secret
- `issuer` - The Cognito Issuer URL (e.g., `https://cognito-idp.us-east-1.amazonaws.com/us-east-1_YourPoolId`)
- `authEndpoint` - The Cognito authorization endpoint
- `tokenEndpoint` - The Cognito token endpoint
- `userInfoEndpoint` - The Cognito user info endpoint
- `endSessionEndpoint` - (Optional) The Cognito logout endpoint
- `rancherUrl` - The Rancher server URL for OAuth callbacks
- `scope` - OAuth scopes (e.g., "openid profile email")
- `groupsClaim` - The claim containing user groups (e.g., "cognito:groups")
- `nameClaim` - The claim containing the user's display name
- `emailClaim` - The claim containing the user's email

The Cognito provider is implemented in `/pkg/auth/providers/cognito/` and extends the generic OIDC provider implementation.

## Terraform Provider Support

The authentication providers listed above are accessible through the Rancher Management API (v3). To manage these providers using Terraform, you need to use the [terraform-provider-rancher2](https://github.com/rancher/terraform-provider-rancher2) provider.

For Terraform support of specific auth providers, including Amazon Cognito, please refer to or contribute to the `terraform-provider-rancher2` repository. The Terraform provider consumes the Rancher Management API and translates Terraform configurations into appropriate API calls.

### Example Terraform Configuration (Conceptual)

Once implemented in the Terraform provider, configuring Amazon Cognito would look similar to:

```hcl
resource "rancher2_auth_config_cognito" "cognito" {
  enabled                = true
  client_id             = "your-cognito-client-id"
  client_secret         = "your-cognito-client-secret"
  issuer                = "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_YourPoolId"
  auth_endpoint         = "https://your-domain.auth.us-east-1.amazoncognito.com/oauth2/authorize"
  token_endpoint        = "https://your-domain.auth.us-east-1.amazoncognito.com/oauth2/token"
  userinfo_endpoint     = "https://your-domain.auth.us-east-1.amazoncognito.com/oauth2/userInfo"
  rancher_url           = "https://your-rancher-server.example.com"
  scope                 = "openid profile email"
  groups_claim          = "cognito:groups"
  name_claim            = "name"
  email_claim           = "email"
}
```

**Note:** The above is a conceptual example. For actual Terraform resource implementation and usage, please consult the `terraform-provider-rancher2` repository documentation.

## API Documentation

For detailed API specifications and usage of authentication providers, refer to:
- Rancher API documentation at `https://your-rancher-server/v3`
- The API types defined in `/pkg/apis/management.cattle.io/v3/authn_types.go`
- Provider implementations in `/pkg/auth/providers/`

## Contributing

If you want to add support for a new authentication provider or enhance existing ones:

1. **For Rancher Core**: Add the provider implementation to `/pkg/auth/providers/` in this repository
2. **For Terraform Support**: Implement the Terraform resource in the `terraform-provider-rancher2` repository

Both implementations should work together, with the Terraform provider consuming the Rancher Management API.
