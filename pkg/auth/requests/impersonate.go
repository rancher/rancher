package requests

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/steve/pkg/auth"
	authenticationv1 "k8s.io/api/authentication/v1"
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
			canDo, err := h.sar.UserCanImpersonateUser(req, user, reqUser)
			if err != nil {
				return nil, false, err
			} else if !canDo {
				return nil, false, errors.New("not allowed to impersonate user")
			}
			impersonateUser = true
		}

		if len(reqGroup) > 0 {
			for _, g := range reqGroup {
				if slices.Contains(groups, g) {
					//user belongs to the group that's trying to impersonate
					continue
				}
				canDo, err := h.sar.UserCanImpersonateGroup(req, user, g)
				if err != nil {
					return nil, false, err
				} else if !canDo {
					return nil, false, errors.New("not allowed to impersonate group")
				}
				impersonateGroup = true
			}
		}

		if reqExtras != nil && len(reqExtras) > 0 {
			canDo, err := h.sar.UserCanImpersonateExtras(req, user, reqExtras)
			if err != nil {
				return nil, false, err
			} else if !canDo {
				return nil, false, errors.New("not allowed to impersonate extras")
			}
			impersonateExtras = true
		}
	}

	var extras map[string][]string

	if impersonateUser {
		if impersonateUser {
			user = reqUser
		}
		if impersonateGroup {
			groups = reqGroup
		} else {
			groups = nil
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
		return encodedKey // Always record extra strings, even if malformed/unencoded.
	}
	return key
}
