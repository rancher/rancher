package tunnelserver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rancher/rancher/pkg/remotedialer"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func NewProxyTunnelServer(ready func() bool, authorizer *Authorizer, server *remotedialer.Server) *remotedialer.ProxyServer {
	return remotedialer.NewProxyServer(authorizer.authorizeProxyTunnel, func(rw http.ResponseWriter, req *http.Request, code int, err error) {
		rw.WriteHeader(code)
		rw.Write([]byte(err.Error()))
	}, server, ready)
}

func (a *Authorizer) authorizeProxyTunnel(req *http.Request) (string, string, bool, error) {
	token := req.Header.Get(Token)
	if token == "" {
		return "", "", false, nil
	}
	params := req.Header.Get(Params)
	if params == "" {
		return "", "", false, nil
	}
	bytes, err := base64.StdEncoding.DecodeString(params)
	if err != nil {
		return "", "", false, nil
	}

	paramsMap := map[string]string{}
	if err := json.Unmarshal(bytes, &paramsMap); err != nil {
		return "", "", false, err
	}
	clusterName, ok := paramsMap["clusterName"]
	if !ok {
		return "", "", false, nil
	}

	requirement, err := labels.NewRequirement(token, selection.Exists, []string{})
	if err != nil {
		return "", "", false, err
	}
	newSelector := labels.NewSelector()
	newSelector = newSelector.Add(*requirement)

	instances, err := a.cattleInstanceLister.List("", newSelector)
	if err != nil {
		return "", "", false, err
	}
	if len(instances) != 1 {
		return "", "", false, errors.New("multiple identity for one token")
	}
	return clusterName, instances[0].Identity, true, nil
}
