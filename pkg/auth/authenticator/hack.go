package authenticator

import (
	"encoding/base64"
	"net/http"
	"strings"
)

type hack struct{}

func (a *hack) Authenticate(req *http.Request) (bool, string, []string, error) {
	user, groupsIMeanPassword, ok := req.BasicAuth()
	if ok {
		parts := strings.Split(groupsIMeanPassword, ",")
		groups := []string{"system:authenticated"}
		groups = append(groups, parts...)
		return true, user, groups, nil
	}

	authCookie, err := req.Cookie("Authentication")
	if err != nil {
		if err == http.ErrNoCookie {
			return false, "", nil, nil
		}
		return false, "", nil, err
	}

	bytes, err := base64.StdEncoding.DecodeString(authCookie.Value)
	if err != nil {
		return false, "", nil, err
	}

	parts := strings.SplitN(string(bytes), ":", 2)
	user = parts[0]
	groups := []string{"system:authenticated"}
	if len(parts) == 2 {
		groups = append(groups, strings.Split(parts[1], ",")...)
	}
	return true, user, groups, nil
}
