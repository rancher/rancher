package requests

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/auth/util"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/wrangler"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type ImpersonatingAuth struct {
	extTokenStore *exttokenstore.SystemStore
	sar           sar.SubjectAccessReview
}

func NewImpersonatingAuth(wranglerContext *wrangler.Context, sar sar.SubjectAccessReview) *ImpersonatingAuth {
	return &ImpersonatingAuth{
		extTokenStore: exttokenstore.NewSystemFromWrangler(wranglerContext),
		sar:           sar,
	}
}

func (i *ImpersonatingAuth) ImpersonationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		userInfo, authed := request.UserFrom(req.Context())
		if !authed {
			util.WriteError(rw, http.StatusUnauthorized, ErrMustAuthenticate)
			return
		}
		user := userInfo.GetName()
		groups := userInfo.GetGroups()

		reqUser := req.Header.Get("Impersonate-User")
		var reqGroup []string
		if g, ok := req.Header["Impersonate-Group"]; ok {
			reqGroup = g
		}
		var reqExtras = impersonateExtrasFromHeaders(req.Header)

		auditUser, ok := audit.FromContext(req.Context())
		if ok {
			auditUser.RequestUser = reqUser
			auditUser.RequestGroups = reqGroup
		}

		// If there is an impersonate header, the incoming request is attempting to
		// impersonate a different user, verify the token user is authz to impersonate
		if i.sar != nil && reqUser != "" && reqUser != user {
			if strings.HasPrefix(reqUser, serviceaccount.ServiceAccountUsernamePrefix) {
				canDo, err := i.sar.UserCanImpersonateServiceAccount(req, user, reqUser)
				if err != nil {
					util.WriteError(rw, http.StatusForbidden, fmt.Errorf("error checking if user can impersonate service account: %w", err))
					return
				} else if !canDo {
					util.WriteError(rw, http.StatusForbidden, fmt.Errorf("not allowed to impersonate service account"))
					return
				}
				// add impersonated SA to context
				*req = *req.WithContext(authcontext.SetSAImpersonation(req.Context(), reqUser))
			} else {
				canDo, err := i.sar.UserCanImpersonateUser(req, user, reqUser)
				if err != nil {
					util.WriteError(rw, http.StatusForbidden, fmt.Errorf("error checking if user can impersonate user: %w", err))
					return
				} else if !canDo {
					util.WriteError(rw, http.StatusForbidden, fmt.Errorf("not allowed to impersonate user"))
					return
				}

				for _, g := range reqGroup {
					if slices.Contains(groups, g) {
						//user belongs to the group they are trying to impersonate
						continue
					}
					canDo, err := i.sar.UserCanImpersonateGroup(req, user, g)
					if err != nil {
						util.WriteError(rw, http.StatusForbidden, fmt.Errorf("error checking if user can impersonate group: %w", err))
						return
					} else if !canDo {
						util.WriteError(rw, http.StatusForbidden, fmt.Errorf("not allowed to impersonate group"))
						return
					}
				}

				if len(reqExtras) > 0 {
					canDo, err := i.sar.UserCanImpersonateExtras(req, user, reqExtras)
					if err != nil {
						util.WriteError(rw, http.StatusForbidden, fmt.Errorf("error checking if user can impersonate extras: %w", err))
						return
					} else if !canDo {
						util.WriteError(rw, http.StatusForbidden, fmt.Errorf("not allowed to impersonate extras"))
						return
					}

					switch requestTokenID := reqExtras[common.ExtraRequestTokenID]; len(requestTokenID) {
					case 0: // Nothing to do.
					case 1:
						token, err := i.extTokenStore.Fetch(requestTokenID[0])
						if err != nil {
							util.WriteError(rw, http.StatusForbidden, fmt.Errorf("error getting request token: %w", err))
							return
						}
						if token.GetUserID() != reqUser {
							util.WriteError(rw, http.StatusForbidden,
								fmt.Errorf("request token user does not match impersonation user"))
							return
						}
					default:
						util.WriteError(rw, http.StatusForbidden, fmt.Errorf("multiple requesttokenid values"))
						return
					}
				}
				reqGroup = append(reqGroup, k8sUser.AllAuthenticated)

				userInfo := &k8sUser.DefaultInfo{
					Name:   reqUser,
					UID:    reqUser,
					Groups: reqGroup,
					Extra:  reqExtras,
				}
				*req = *req.WithContext(request.WithUser(req.Context(), userInfo))
			}
		}

		next.ServeHTTP(rw, req)
	})
}

func impersonateExtrasFromHeaders(headers http.Header) map[string][]string {
	var extras map[string][]string
	for headerName, values := range headers {
		if !strings.HasPrefix(headerName, authenticationv1.ImpersonateUserExtraHeaderPrefix) {
			continue
		}
		extraKey := unescapeExtraKey(strings.ToLower(headerName[len(authenticationv1.ImpersonateUserExtraHeaderPrefix):]))
		if extras == nil {
			extras = make(map[string][]string)
		}
		extras[extraKey] = values
	}

	return extras
}

func unescapeExtraKey(encodedKey string) string {
	key, err := url.PathUnescape(encodedKey) // Decode %-encoded bytes.
	if err != nil {
		return encodedKey // Always try even if unencoded.
	}
	return key
}
