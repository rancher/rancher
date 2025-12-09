package rbac

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/server"
	"github.com/sirupsen/logrus"
	k8sSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type SchemaRbacProcessor struct {
	next      http.Handler
	apiServer http.Handler
}

func (s *SchemaRbacProcessor) Process(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Request URI: %s matched an RBAC filtering rule.", r.URL.RequestURI())

	middleWriter := MiddleResponseWriter(w)
	s.next.ServeHTTP(middleWriter, r)

	body := middleWriter.GetResponseBody()

	isGzip := middleWriter.Header().Get("Content-Encoding") == "gzip"

	if isGzip {
		decompressed, err := decompressGzip(body)
		if err != nil {
			logrus.Warnf("Failed to decompress gzip: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body = decompressed
	}

	filteredBody := filterResponse(r, body, s.apiServer)

	// Re-compress the body if the original response was gzip
	if isGzip {
		compressed, err := compressGzip(filteredBody)
		if err != nil {
			logrus.Warnf("Failed to compress gzip: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		filteredBody = compressed
	}

	// Write filtered response back to the client
	for k, vv := range middleWriter.Header() {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	status := middleWriter.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	w.Write(filteredBody)

}

func (s *SchemaRbacProcessor) Match(r *http.Request) bool {
	url := r.URL.RequestURI()
	pattern := `^/k8s/clusters/[^/]+/v1/(harvester/)?schemas(\?.*)?$`
	matched, err := regexp.MatchString(pattern, url)
	if err != nil {
		return false
	}
	return matched
}

type HijackResponseWriter struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	StatusCode int
	HeaderMap  http.Header
	Conn       net.Conn
	Reader     io.Reader
	Writer     io.Writer
}

func MiddleResponseWriter(w http.ResponseWriter) *HijackResponseWriter {
	return &HijackResponseWriter{
		ResponseWriter: w,
		Body:           &bytes.Buffer{},
		StatusCode:     http.StatusOK,
		HeaderMap:      make(http.Header),
	}
}

// Write captures the response body
func (c *HijackResponseWriter) Write(p []byte) (int, error) {
	c.Body.Write(p)
	return len(p), nil
}

func (c *HijackResponseWriter) WriteHeader(statusCode int) {
	c.StatusCode = statusCode
}

func (c *HijackResponseWriter) Header() http.Header {
	return c.HeaderMap
}

func (c *HijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := c.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	c.Conn = conn
	c.Reader = rw.Reader
	c.Writer = rw.Writer
	return conn, rw, nil
}

func (c *HijackResponseWriter) GetResponseBody() []byte {
	return c.Body.Bytes()
}

// filterResponse filters schemas based on accessSet; if any error occurs, return an empty result to prevent privilege escalation
func filterResponse(r *http.Request, dataToFilter []byte, apiServer http.Handler) []byte {
	var finalResult = make([]byte, 0)

	user, ok := request.UserFrom(r.Context())

	if !ok {
		logrus.Warnf("Unable to find a user in the current request context!")
		return finalResult
	}

	if user.GetExtra()["username"][0] == "admin" {
		return dataToFilter
	}

	steve, _ := apiServer.(*server.Server)

	c := schema.NewCollection(nil, steve.BaseSchemas, steve.AccessSetLookup)
	schemas, err := c.Schemas(user)
	if err != nil {
		return finalResult
	}
	accessSet, ok := schemas.Attributes["accessSet"].(*accesscontrol.AccessSet)
	if !ok {
		logrus.Warnf("User %s does not have an accessSet!", user.GetName())
		return finalResult
	}

	// Parse raw JSON
	var raw map[string]interface{}
	if err = json.Unmarshal(dataToFilter, &raw); err != nil {
		return finalResult
	}

	items, ok := raw["data"].([]interface{})
	if !ok {
		return finalResult
	}

	var filtered []interface{}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		attr, ok := m["attributes"].(map[string]interface{})
		if !ok {
			// Schema does not require RBAC check; include it directly
			filtered = append(filtered, m)
			continue
		}

		group := ""
		if g, ok := attr["group"].(string); ok {
			group = g
		}

		resource := ""
		if r, ok := attr["resource"].(string); ok {
			resource = r
		}

		gr := k8sSchema.GroupResource{
			Group:    group,
			Resource: resource,
		}

		ns := ""
		if n, ok := attr["namespace"].(string); ok {
			ns = n
		}

		name := ""
		if n, ok := attr["name"].(string); ok {
			name = n
		}

		// Filter resourceMethods
		var filteredR []interface{}
		if rMethods, ok := m["resourceMethods"].([]interface{}); ok && rMethods != nil {
			for _, v := range rMethods {
				if verb, ok := v.(string); ok {
					verb = strings.ToLower(verb)
					if accessSet.Grants(verb, gr, ns, name) {
						filteredR = append(filteredR, verb)
					}
				}
			}
		}
		m["resourceMethods"] = filteredR

		// Filter collectionMethods
		var filteredC []interface{}
		if cMethods, ok := m["collectionMethods"].([]interface{}); ok && cMethods != nil {
			for _, v := range cMethods {
				if verb, ok := v.(string); ok {
					verb = strings.ToLower(verb)
					if accessSet.Grants(verb, gr, accesscontrol.All, accesscontrol.All) {
						filteredC = append(filteredC, verb)
					}
				}
			}
		}
		m["collectionMethods"] = filteredC

		// If both sets are empty, remove the entire resource
		if len(filteredR) == 0 && len(filteredC) == 0 {
			continue
		}

		filtered = append(filtered, m)
	}

	raw["data"] = filtered
	finalResult, _ = json.Marshal(raw)
	return finalResult
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		logrus.Warnf("Error closing the writer while compressing data: %v!", err)
	}
	return buf.Bytes(), nil
}

func decompressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
