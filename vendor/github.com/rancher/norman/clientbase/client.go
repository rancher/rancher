package clientbase

import (
	"net/http"

	"github.com/rancher/norman/types"
)

type APIBaseClient struct {
	Ops   *APIOperations
	Opts  *ClientOpts
	Types map[string]types.Schema
}

type APIOperations struct {
	Opts   *ClientOpts
	Types  map[string]types.Schema
	Client *http.Client
}
