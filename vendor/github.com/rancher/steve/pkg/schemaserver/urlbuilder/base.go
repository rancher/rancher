package urlbuilder

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func ParseRequestURL(r *http.Request) string {
	scheme := GetScheme(r)
	host := GetHost(r, scheme)
	return fmt.Sprintf("%s://%s%s%s", scheme, host, r.Header.Get(PrefixHeader), r.URL.Path)
}

func GetHost(r *http.Request, scheme string) string {
	host := r.Header.Get(ForwardedAPIHostHeader)
	if host != "" {
		return host
	}

	host = strings.Split(r.Header.Get(ForwardedHostHeader), ",")[0]
	if host != "" {
		return host
	}

	return r.Host
}

func GetScheme(r *http.Request) string {
	scheme := r.Header.Get(ForwardedProtoHeader)
	if scheme != "" {
		switch scheme {
		case "ws":
			return "http"
		case "wss":
			return "https"
		default:
			return scheme
		}
	} else if r.TLS != nil {
		return "https"
	}
	return "http"
}

func ParseResponseURLBase(currentURL string, r *http.Request) (string, error) {
	path := r.URL.Path

	index := strings.LastIndex(currentURL, path)
	if index == -1 {
		// Fallback, if we can't find path in currentURL, then we just assume the base is the root of the web request
		u, err := url.Parse(currentURL)
		if err != nil {
			return "", err
		}

		buffer := bytes.Buffer{}
		buffer.WriteString(u.Scheme)
		buffer.WriteString("://")
		buffer.WriteString(u.Host)
		return buffer.String(), nil
	}

	return currentURL[0:index], nil
}
