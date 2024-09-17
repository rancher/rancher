package requests

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/steve/pkg/auth"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	k8sUser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type impersonatingAuth struct {
	sar sar.SubjectAccessReview
}

func NewImpersonatingAuth(sar sar.SubjectAccessReview) auth.Authenticator {
	return &impersonatingAuth{
		sar: sar,
	}
}

func (h *impersonatingAuth) Authenticate(req *http.Request) (k8sUser.Info, bool, error) {
	userInfo, authed := request.UserFrom(req.Context())
	if !authed {
		return nil, false, nil
	}
	user := userInfo.GetName()
	groups := userInfo.GetGroups()

	var impersonateUser bool
	var impersonateGroup bool
	var impersonateExtras bool

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
	if h.sar != nil {
		if reqUser != "" && reqUser != user {
			if strings.HasPrefix(reqUser, serviceaccount.ServiceAccountUsernamePrefix) {
				canDo, err := h.sar.UserCanImpersonateServiceAccount(req, user, reqUser)
				if err != nil {
					return nil, false, fmt.Errorf("error checking if user can impersonate service account: %w", err)
				} else if !canDo {
					return nil, false, errors.New("not allowed to impersonate service account")
				}
				// add impersonated SA to context
				*req = *req.WithContext(authcontext.SetSAImpersonation(req.Context(), reqUser))
			} else {
				canDo, err := h.sar.UserCanImpersonateUser(req, user, reqUser)
				if err != nil {
					return nil, false, fmt.Errorf("error checking if user can impersonate another user: %w", err)
				} else if !canDo {
					return nil, false, errors.New("not allowed to impersonate user")
				}
				impersonateUser = true

				for _, g := range reqGroup {
					if slices.Contains(groups, g) {
						//user belongs to the group they are trying to impersonate
						continue
					}
					canDo, err := h.sar.UserCanImpersonateGroup(req, user, g)
					if err != nil {
						return nil, false, fmt.Errorf("error checking if user can impersonate group: %w", err)
					} else if !canDo {
						return nil, false, errors.New("not allowed to impersonate group")
					}
					impersonateGroup = true
				}

				if len(reqExtras) > 0 {
					canDo, err := h.sar.UserCanImpersonateExtras(req, user, reqExtras)
					if err != nil {
						return nil, false, fmt.Errorf("error checking if user can impersonate extras: %w", err)
					} else if !canDo {
						return nil, false, errors.New("not allowed to impersonate extras")
					}
					impersonateExtras = true
				}
			}
		}
	}

	var extras map[string][]string

	if impersonateUser {
		user = reqUser
		groups = nil

		if impersonateGroup {
			groups = reqGroup
		}
		if impersonateExtras {
			extras = reqExtras
		}
		groups = append(groups, k8sUser.AllAuthenticated)
	} else {
		extras = userInfo.GetExtra()
	}

	return &k8sUser.DefaultInfo{
		Name:   user,
		UID:    user,
		Groups: groups,
		Extra:  extras,
	}, true, nil
}

func impersonateExtrasFromHeaders(headers http.Header) map[string][]string {
	extras := make(map[string][]string)
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
