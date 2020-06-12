package parse

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/types"
)

type Vars struct {
	Type      string
	Name      string
	Namespace string
	Link      string
	Prefix    string
	Action    string
}

func Set(v Vars) mux.MatcherFunc {
	return func(request *http.Request, match *mux.RouteMatch) bool {
		if match.Vars == nil {
			match.Vars = map[string]string{}
		}
		if v.Type != "" {
			match.Vars["type"] = v.Type
		}
		if v.Name != "" {
			match.Vars["name"] = v.Name
		}
		if v.Link != "" {
			match.Vars["link"] = v.Link
		}
		if v.Prefix != "" {
			match.Vars["prefix"] = v.Prefix
		}
		if v.Action != "" {
			match.Vars["action"] = v.Action
		}
		if v.Namespace != "" {
			match.Vars["namespace"] = v.Namespace
		}
		return true
	}
}

func MuxURLParser(rw http.ResponseWriter, req *http.Request, schemas *types.APISchemas) (ParsedURL, error) {
	vars := mux.Vars(req)
	url := ParsedURL{
		Type:      vars["type"],
		Name:      vars["name"],
		Namespace: vars["namespace"],
		Link:      vars["link"],
		Prefix:    vars["prefix"],
		Method:    req.Method,
		Action:    vars["action"],
		Query:     req.URL.Query(),
	}

	return url, nil
}
