package parse

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/steve/pkg/schemaserver/types"
)

func MuxURLParser(rw http.ResponseWriter, req *http.Request, schemas *types.APISchemas) (ParsedURL, error) {
	vars := mux.Vars(req)
	url := ParsedURL{
		Type:   vars["type"],
		Name:   vars["name"],
		Link:   vars["link"],
		Prefix: vars["prefix"],
		Method: req.Method,
		Action: vars["action"],
		Query:  req.URL.Query(),
	}

	return url, nil
}
