package httpproxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	ForwardProto = "X-Forwarded-Proto"
	APIAuth      = "X-API-Auth-Header"
)

var (
	httpStart  = regexp.MustCompile("^http:/([^/])")
	httpsStart = regexp.MustCompile("^https:/([^/])")
	badHeaders = map[string]bool{
		"host":              true,
		"transfer-encoding": true,
		"content-length":    true,
		"x-api-auth-header": true,
	}
)

type Supplier func() []string

type proxy struct {
	prefix             string
	validHostsSupplier Supplier
}

func (p *proxy) isAllowed(host string) bool {
	for _, valid := range p.validHostsSupplier() {
		if valid == host {
			return true
		}

		if strings.HasPrefix(valid, "*") && strings.HasSuffix(host, valid[1:]) {
			return true
		}
	}

	return false
}

func NewProxy(prefix string, validHosts Supplier) http.Handler {
	p := proxy{
		prefix:             prefix,
		validHostsSupplier: validHosts,
	}

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if err := p.proxy(req); err != nil {
				logrus.Infof("Failed to proxy %v: %v", req, err)
			}
		},
	}
}

func (p *proxy) proxy(req *http.Request) error {
	path := req.URL.String()
	index := strings.Index(path, p.prefix)
	destPath := path[index+len(p.prefix):]

	if httpsStart.MatchString(destPath) {
		destPath = httpsStart.ReplaceAllString(destPath, "https://$1")
	} else if httpStart.MatchString(destPath) {
		destPath = httpStart.ReplaceAllString(destPath, "http://$1")
	} else {
		destPath = "https://" + destPath
	}

	destURL, err := url.Parse(destPath)
	if err != nil {
		return err
	}

	destURL.RawQuery = req.URL.RawQuery

	if !p.isAllowed(destURL.Host) {
		return fmt.Errorf("invalid host: %v", destURL.Host)
	}

	headerCopy := http.Header{}

	if req.TLS != nil {
		headerCopy.Set(ForwardProto, "https")
	}

	auth := req.Header.Get(APIAuth)
	if auth != "" {
		headerCopy.Set("Authorization", auth)
	}

	for name, value := range req.Header {
		if badHeaders[strings.ToLower(name)] {
			continue
		}

		copy := make([]string, len(value))
		for i := range value {
			copy[i] = strings.TrimPrefix(value[i], "rancher:")
		}
		headerCopy[name] = copy
	}

	req.Host = destURL.Hostname()
	req.URL = destURL
	req.Header = headerCopy

	return nil
}
