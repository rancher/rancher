package clientbase

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
)

const (
	SELF       = "self"
	COLLECTION = "collection"
)

var (
	debug  = false
	dialer = &websocket.Dialer{}
)

type ClientOpts struct {
	URL        string
	AccessKey  string
	SecretKey  string
	Timeout    time.Duration
	HTTPClient *http.Client
	CACerts    string
}

type APIError struct {
	StatusCode int
	URL        string
	Msg        string
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	return e.Msg
}

func IsNotFound(err error) bool {
	apiError, ok := err.(*APIError)
	if !ok {
		return false
	}

	return apiError.StatusCode == http.StatusNotFound
}

func newAPIError(resp *http.Response, url string) *APIError {
	contents, err := ioutil.ReadAll(resp.Body)
	var body string
	if err != nil {
		body = "Unreadable body."
	} else {
		body = string(contents)
	}

	data := map[string]interface{}{}
	if json.Unmarshal(contents, &data) == nil {
		delete(data, "id")
		delete(data, "links")
		delete(data, "actions")
		delete(data, "type")
		delete(data, "status")
		buf := &bytes.Buffer{}
		for k, v := range data {
			if v == nil {
				continue
			}
			if buf.Len() > 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(buf, "%s=%v", k, v)
		}
		body = buf.String()
	}
	formattedMsg := fmt.Sprintf("Bad response statusCode [%d]. Status [%s]. Body: [%s] from [%s]",
		resp.StatusCode, resp.Status, body, url)
	return &APIError{
		URL:        url,
		Msg:        formattedMsg,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       body,
	}
}

func contains(array []string, item string) bool {
	for _, check := range array {
		if check == item {
			return true
		}
	}

	return false
}

func appendFilters(urlString string, filters map[string]interface{}) (string, error) {
	if len(filters) == 0 {
		return urlString, nil
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range filters {
		if l, ok := v.([]string); ok {
			for _, v := range l {
				q.Add(k, v)
			}
		} else {
			q.Add(k, fmt.Sprintf("%v", v))
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func NewAPIClient(opts *ClientOpts) (APIBaseClient, error) {
	var err error

	result := APIBaseClient{
		Types: map[string]types.Schema{},
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{}
	}

	if opts.Timeout == 0 {
		opts.Timeout = time.Second * 10
	}

	client.Timeout = opts.Timeout

	if opts.CACerts != "" {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM([]byte(opts.CACerts))
		if !ok {
			return result, err
		}
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		}
		client.Transport = tr
	}

	req, err := http.NewRequest("GET", opts.URL, nil)
	if err != nil {
		return result, err
	}

	req.SetBasicAuth(opts.AccessKey, opts.SecretKey)

	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return result, newAPIError(resp, opts.URL)
	}

	schemasURLs := resp.Header.Get("X-API-Schemas")
	if len(schemasURLs) == 0 {
		return result, errors.New("Failed to find schema at [" + opts.URL + "]")
	}

	if schemasURLs != opts.URL {
		req, err = http.NewRequest("GET", schemasURLs, nil)
		req.SetBasicAuth(opts.AccessKey, opts.SecretKey)
		if err != nil {
			return result, err
		}

		resp, err = client.Do(req)
		if err != nil {
			return result, err
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return result, newAPIError(resp, opts.URL)
		}
	}

	var schemas types.SchemaCollection
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	err = json.Unmarshal(bytes, &schemas)
	if err != nil {
		return result, err
	}

	for _, schema := range schemas.Data {
		result.Types[schema.ID] = schema
	}

	result.Opts = opts
	result.Ops = &APIOperations{
		Opts:   opts,
		Client: client,
		Types:  result.Types,
	}

	return result, nil
}

func NewListOpts() *types.ListOpts {
	return &types.ListOpts{
		Filters: map[string]interface{}{},
	}
}

func (a *APIBaseClient) Websocket(url string, headers map[string][]string) (*websocket.Conn, *http.Response, error) {
	httpHeaders := http.Header{}
	for k, v := range httpHeaders {
		httpHeaders[k] = v
	}

	if a.Opts != nil {
		s := a.Opts.AccessKey + ":" + a.Opts.SecretKey
		httpHeaders.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(s)))
	}

	return dialer.Dial(url, http.Header(httpHeaders))
}

func (a *APIBaseClient) List(schemaType string, opts *types.ListOpts, respObject interface{}) error {
	return a.Ops.DoList(schemaType, opts, respObject)
}

func (a *APIBaseClient) Post(url string, createObj interface{}, respObject interface{}) error {
	return a.Ops.DoModify("POST", url, createObj, respObject)
}

func (a *APIBaseClient) GetLink(resource types.Resource, link string, respObject interface{}) error {
	url := resource.Links[link]
	if url == "" {
		return fmt.Errorf("failed to find link: %s", link)
	}

	return a.Ops.DoGet(url, &types.ListOpts{}, respObject)
}

func (a *APIBaseClient) Create(schemaType string, createObj interface{}, respObject interface{}) error {
	return a.Ops.DoCreate(schemaType, createObj, respObject)
}

func (a *APIBaseClient) Update(schemaType string, existing *types.Resource, updates interface{}, respObject interface{}) error {
	return a.Ops.DoUpdate(schemaType, existing, updates, respObject)
}

func (a *APIBaseClient) ByID(schemaType string, id string, respObject interface{}) error {
	return a.Ops.DoByID(schemaType, id, respObject)
}

func (a *APIBaseClient) Delete(existing *types.Resource) error {
	if existing == nil {
		return nil
	}
	return a.Ops.DoResourceDelete(existing.Type, existing)
}

func (a *APIBaseClient) Reload(existing *types.Resource, output interface{}) error {
	selfURL, ok := existing.Links[SELF]
	if !ok {
		return fmt.Errorf("failed to find self URL of [%v]", existing)
	}

	return a.Ops.DoGet(selfURL, NewListOpts(), output)
}

func (a *APIBaseClient) Action(schemaType string, action string,
	existing *types.Resource, inputObject, respObject interface{}) error {
	return a.Ops.DoAction(schemaType, action, existing, inputObject, respObject)
}

func init() {
	debug = os.Getenv("RANCHER_CLIENT_DEBUG") == "true"
	if debug {
		fmt.Println("Rancher client debug on")
	}
}
