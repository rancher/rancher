package server

import (
	"net/http"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type Server interface {
	Close()
	Handler() http.Handler
	Cluster() *v3.Cluster
}
