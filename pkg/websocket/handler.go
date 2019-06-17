package websocket

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/norman/httperror"
)

func NewWebsocketHandler(handler http.Handler) http.Handler {
	return &websocketHandler{
		handler,
	}
}

type websocketHandler struct {
	next http.Handler
}

func (h websocketHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isWebsocket(req) {
		if !checkSameOrigin(req) {
			response(rw, httperror.PermissionDenied, "origin not allowed")
			return
		}
	}
	h.next.ServeHTTP(rw, req)
}

// Inspired by https://github.com/gorilla/websocket/blob/80c2d40e9b91f2ef7a9c1a403aeec64d1b89a9a6/server.go#L87
// checkSameOrigin returns true if the origin is not set or is equal to the request host.
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(origin[0])
	if err != nil {
		return false
	}

	if u.Port() == "" {
		return u.Host == r.Host
	}
	return u.Host == r.Host && u.Port() == portOnly(r.Host)
}

// isWebsocket returns true if the request is a websocket
func isWebsocket(r *http.Request) bool {
	if !headerListContainsValue(r.Header, "Connection", "upgrade") {
		return false
	}
	return true
}

// headerListContainsValue returns true if the token header with the given name contains token.
func headerListContainsValue(header http.Header, name string, value string) bool {
	for _, v := range header[name] {
		for _, s := range strings.Split(v, ",") {
			if strings.EqualFold(value, strings.TrimSpace(s)) {
				return true
			}
		}
	}
	return false
}

func response(rw http.ResponseWriter, code httperror.ErrorCode, message string) {
	rw.WriteHeader(code.Status)
	rw.Header().Set("content-type", "application/json")
	json.NewEncoder(rw).Encode(httperror.NewAPIError(code, message))
}

// portOnly returns the port part of localhost:port, without the leading colon
func portOnly(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return ""
	}
	if i := strings.Index(hostport, "]:"); i != -1 {
		return hostport[i+len("]:"):]
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return hostport[colon+len(":"):]
}
