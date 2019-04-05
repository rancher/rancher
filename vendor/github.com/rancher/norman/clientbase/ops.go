package clientbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
)

type APIOperations struct {
	Opts   *ClientOpts
	Types  map[string]types.Schema
	Client *http.Client
	Dialer *websocket.Dialer
}

func (a *APIOperations) SetupRequest(req *http.Request) {
	req.Header.Add("Authorization", a.Opts.getAuthHeader())
}

func (a *APIOperations) DoDelete(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	a.SetupRequest(req)

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return NewAPIError(resp, url)
	}

	return nil
}

func (a *APIOperations) DoGet(url string, opts *types.ListOpts, respObject interface{}) error {
	if opts == nil {
		opts = NewListOpts()
	}
	url, err := appendFilters(url, opts.Filters)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Println("GET " + url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	a.SetupRequest(req)

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return NewAPIError(resp, url)
	}

	byteContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Println("Response <= " + string(byteContent))
	}

	if err := json.Unmarshal(byteContent, respObject); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to parse: %s", byteContent))
	}

	return nil
}

func (a *APIOperations) DoList(schemaType string, opts *types.ListOpts, respObject interface{}) error {
	schema, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	if !contains(schema.CollectionMethods, "GET") {
		return errors.New("Resource type [" + schemaType + "] is not listable")
	}

	collectionURL, ok := schema.Links["collection"]
	if !ok {
		return errors.New("Resource type [" + schemaType + "] does not have a collection URL")
	}

	return a.DoGet(collectionURL, opts, respObject)
}

func (a *APIOperations) DoNext(nextURL string, respObject interface{}) error {
	return a.DoGet(nextURL, nil, respObject)
}

func (a *APIOperations) DoModify(method string, url string, createObj interface{}, respObject interface{}) error {
	if createObj == nil {
		createObj = map[string]string{}
	}
	if respObject == nil {
		respObject = &map[string]interface{}{}
	}
	bodyContent, err := json.Marshal(createObj)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Println(method + " " + url)
		fmt.Println("Request => " + string(bodyContent))
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyContent))
	if err != nil {
		return err
	}

	a.SetupRequest(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return NewAPIError(resp, url)
	}

	byteContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(byteContent) > 0 {
		if Debug {
			fmt.Println("Response <= " + string(byteContent))
		}
		return json.Unmarshal(byteContent, respObject)
	}

	return nil
}

func (a *APIOperations) DoCreate(schemaType string, createObj interface{}, respObject interface{}) error {
	if createObj == nil {
		createObj = map[string]string{}
	}
	if respObject == nil {
		respObject = &map[string]interface{}{}
	}
	schema, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	if !contains(schema.CollectionMethods, "POST") {
		return errors.New("Resource type [" + schemaType + "] is not creatable")
	}

	var collectionURL string
	collectionURL, ok = schema.Links[COLLECTION]
	if !ok {
		// return errors.New("Failed to find collection URL for [" + schemaType + "]")
		// This is a hack to address https://github.com/rancher/cattle/issues/254
		re := regexp.MustCompile("schemas.*")
		collectionURL = re.ReplaceAllString(schema.Links[SELF], schema.PluralName)
	}

	return a.DoModify("POST", collectionURL, createObj, respObject)
}

func (a *APIOperations) DoReplace(schemaType string, existing *types.Resource, updates interface{}, respObject interface{}) error {
	return a.doUpdate(schemaType, true, existing, updates, respObject)
}

func (a *APIOperations) DoUpdate(schemaType string, existing *types.Resource, updates interface{}, respObject interface{}) error {
	return a.doUpdate(schemaType, false, existing, updates, respObject)
}

func (a *APIOperations) doUpdate(schemaType string, replace bool, existing *types.Resource, updates interface{}, respObject interface{}) error {
	if existing == nil {
		return errors.New("Existing object is nil")
	}

	selfURL, ok := existing.Links[SELF]
	if !ok {
		return fmt.Errorf("failed to find self URL of [%v]", existing)
	}

	if replace {
		u, err := url.Parse(selfURL)
		if err != nil {
			return fmt.Errorf("failed to parse url %s: %v", selfURL, err)
		}
		q := u.Query()
		q.Set("_replace", "true")
		u.RawQuery = q.Encode()
		selfURL = u.String()
	}

	if updates == nil {
		updates = map[string]string{}
	}

	if respObject == nil {
		respObject = &map[string]interface{}{}
	}

	schema, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	if !contains(schema.ResourceMethods, "PUT") {
		return errors.New("Resource type [" + schemaType + "] is not updatable")
	}

	return a.DoModify("PUT", selfURL, updates, respObject)
}

func (a *APIOperations) DoByID(schemaType string, id string, respObject interface{}) error {
	schema, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	if !contains(schema.ResourceMethods, "GET") {
		return errors.New("Resource type [" + schemaType + "] can not be looked up by ID")
	}

	collectionURL, ok := schema.Links[COLLECTION]
	if !ok {
		return errors.New("Failed to find collection URL for [" + schemaType + "]")
	}

	return a.DoGet(collectionURL+"/"+id, nil, respObject)
}

func (a *APIOperations) DoResourceDelete(schemaType string, existing *types.Resource) error {
	schema, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	if !contains(schema.ResourceMethods, "DELETE") {
		return errors.New("Resource type [" + schemaType + "] can not be deleted")
	}

	selfURL, ok := existing.Links[SELF]
	if !ok {
		return fmt.Errorf("failed to find self URL of [%v]", existing)
	}

	return a.DoDelete(selfURL)
}

func (a *APIOperations) DoAction(schemaType string, action string,
	existing *types.Resource, inputObject, respObject interface{}) error {

	if existing == nil {
		return errors.New("Existing object is nil")
	}

	actionURL, ok := existing.Actions[action]
	if !ok {
		return fmt.Errorf("action [%v] not available on [%v]", action, existing)
	}

	return a.doAction(schemaType, action, actionURL, inputObject, respObject)
}

func (a *APIOperations) DoCollectionAction(schemaType string, action string,
	existing *types.Collection, inputObject, respObject interface{}) error {

	if existing == nil {
		return errors.New("Existing object is nil")
	}

	actionURL, ok := existing.Actions[action]
	if !ok {
		return fmt.Errorf("action [%v] not available on [%v]", action, existing)
	}

	return a.doAction(schemaType, action, actionURL, inputObject, respObject)
}

func (a *APIOperations) doAction(
	schemaType string,
	action string,
	actionURL string,
	inputObject interface{},
	respObject interface{},
) error {
	_, ok := a.Types[schemaType]
	if !ok {
		return errors.New("Unknown schema type [" + schemaType + "]")
	}

	var input io.Reader

	if Debug {
		fmt.Println("POST " + actionURL)
	}

	if inputObject != nil {
		bodyContent, err := json.Marshal(inputObject)
		if err != nil {
			return err
		}
		if Debug {
			fmt.Println("Request => " + string(bodyContent))
		}
		input = bytes.NewBuffer(bodyContent)
	}

	req, err := http.NewRequest("POST", actionURL, input)
	if err != nil {
		return err
	}

	a.SetupRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "0")

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return NewAPIError(resp, actionURL)
	}

	byteContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Println("Response <= " + string(byteContent))
	}

	if nil != respObject {
		return json.Unmarshal(byteContent, respObject)
	}
	return nil
}
