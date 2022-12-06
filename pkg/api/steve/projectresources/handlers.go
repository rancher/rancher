// Package projectresources supports the /apis/resources.project.cattle.io/v1alpha1 endpoint for listing resources by project.
package projectresources

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	workers                    = int64(3)
	notOp                      = "!"
	extensionConfigMap         = "extension-apiserver-authentication"
	kubeSystemNamespace        = "kube-system"
	clientCAKey                = "requestheader-client-ca-file"
	serverAllowedCNKey         = "requestheader-allowed-names"
	projectNamespaceAnnotation = "management.cattle.io/system-namespace"
)

var (
	errUnsupportedAction      = errors.New("this action is not supported")
	errUnsupportedContentType = errors.New("could not negotiate content type")
)

// authenticateAPIServer is a middleware that verifies the client's certificates and checks that the client's CN is in the API server's allow list.
// This fulfills the two conditions outlines in https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/#kubernetes-apiserver-client-authentication:
// > 1. The connection must be made using a client certificate that is signed by the CA whose certificate is in --requestheader-client-ca-file.
// > 2. The connection must be made using a client certificate whose CN is one of those listed in --requestheader-allowed-names.
// The http.Server must have a TLS config with ClientAuth: tls.RequestClientCert set in order for the http.Request to have the client cert populated.
// Normally mTLS would be handled at the network session layer automatically during the client connection when tls.RequireAndVerifyClientCert
// is set on the Server's tls.Config, but since this server performs multiple functions and not all of them require mTLS, we handle it in the
// application layer in this middleware.
func (a *authHandler) authenticateAPIServer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, err := a.configMapCache.Get(kubeSystemNamespace, extensionConfigMap)
		if err != nil {
			logrus.Errorf("[%s] could not authenticate API server: %v, please configure kube-apiserver with --%s and --%s", apiServiceName, err, clientCAKey, serverAllowedCNKey)
			http.Error(w, "could not authenticate API server", http.StatusInternalServerError)
			return
		}

		clientCA, ok := config.Data[clientCAKey]
		if !ok {
			logrus.Errorf("[%s] could not authenticate API server: %v, please configure kube-apiserver with --%s and --%s", apiServiceName, err, clientCAKey, serverAllowedCNKey)
			http.Error(w, "could not authenticate API server", http.StatusInternalServerError)
			return
		}
		allowedCN, ok := config.Data[serverAllowedCNKey]
		if !ok || len(allowedCN) == 0 {
			logrus.Warnf("[%s] key %s in configmap %s/%s is missing or empty, any CN will be accepted for requests to /apis/%s, change this by setting --%s on kube-apiserver",
				apiServiceName, serverAllowedCNKey, kubeSystemNamespace, extensionConfigMap, groupVersion, serverAllowedCNKey)
		}
		var handleError = func(err error) {
			switch err.(type) {
			case authError:
				logrus.Warnf("[%s] could not authenticate API server: %v", apiServiceName, err)
				http.Error(w, fmt.Sprintf("could not authenticate API server: %v", err), http.StatusUnauthorized)
			case configError:
				logrus.Errorf("[%s] could not authenticate API server: %v", apiServiceName, err)
				http.Error(w, fmt.Sprintf("could not authenticate API server: %v", err), http.StatusInternalServerError)
			}
		}
		err = verifyCert(r, clientCA)
		if err != nil {
			handleError(err)
			return
		}
		err = verifyCN(r, allowedCN)
		if err != nil {
			handleError(err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func verifyCert(r *http.Request, clientCA string) error {
	if len(r.TLS.PeerCertificates) == 0 {
		logrus.Errorf("[%s] client did not provide certificate", apiServiceName)
		return authError{"client did not provide certificate"}
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(clientCA))
	opts := x509.VerifyOptions{
		Roots:         caCertPool,
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	for _, cert := range r.TLS.PeerCertificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	_, err := r.TLS.PeerCertificates[0].Verify(opts)
	if err != nil {
		logrus.Errorf("[%s] failed to verify API server client certificate: %v", apiServiceName, err)
		return authError{err.Error()}
	}
	return nil
}

func verifyCN(r *http.Request, allowedCNString string) error {
	if allowedCNString == "" {
		return nil
	}
	allowedCN := []string{}
	if err := json.Unmarshal([]byte(allowedCNString), &allowedCN); err != nil {
		return configError{}
	}
	requestCN := r.TLS.PeerCertificates[0].Subject.CommonName
	found := false
	for _, allowed := range allowedCN {
		if allowed == requestCN {
			found = true
			break
		}
	}
	if !found {
		logrus.Errorf("[%s] could not find user %s in allowed users", apiServiceName, requestCN)
		return authError{"user not allowed"}
	}
	return nil
}

// discoveryHandler is used by the k8s API server to discover and register resources.
// Used by `kubectl api-resources` and steve schema registration.
func (h *handler) discoveryHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("[%s] handling request %s %s", apiServiceName, r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"kind":         "APIResourceList",
		"apiVersion":   "v1",
		"groupVersion": groupVersion,
		"resources":    h.apis.List(),
	}
	apiResourceBytes, err := json.Marshal(resp)
	if err != nil {
		logrus.Errorf("[%s] failed to encode discovery response: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(apiResourceBytes)
}

// forwarder handles global resource requests that have no particular project or namespace specified.
// In this case, there is nothing special about the request so the regular client can handle it without any preprocessing.
func (h *handler) forwarder(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("[%s] handling request %s %s", apiServiceName, r.Method, r.URL.Path)
	vars := mux.Vars(r)
	resource, err := h.gvrFromVars(vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	resourceClient, err := h.clientGetter(r, h.restConfig, resource)
	if errors.Is(err, errUnsupportedContentType) {
		logrus.Warnf("[%s] unsupported content type requested: %s", apiServiceName, r.Header.Get("Accept"))
		http.Error(w, "unsupported content type requested", http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get client for config: %v", apiServiceName, err)
		http.Error(w, "failed to get client for config", http.StatusInternalServerError)
		return
	}
	if !h.hasAccess(r, "", resource) {
		h.returnEmpty(r.Context(), w, resource, resourceClient)
		return
	}
	opts := metav1.ListOptions{}
	err = paramCodec.DecodeParameters(r.URL.Query(), metav1.SchemeGroupVersion, &opts)
	if err != nil {
		logrus.Errorf("[%s] failed to parse query %v: %v", apiServiceName, r.URL.Query(), err)
		http.Error(w, "failed to parse query", http.StatusInternalServerError)
	}
	if opts.Watch {
		// watch is not currently supported. kube-controller-manager starts a
		// watch on all resources, and there is no way to tell it not to. We
		// could start a real watch with the resourceClient, but
		// kube-controller-manager is already getting the real events from the
		// real endpoint, so we don't need to create more work for it by
		// duplicating events.
		logrus.Tracef("[%s] starting dummy watch for resource %s", apiServiceName, resource.Resource)
		for {
			select {
			case <-r.Context().Done():
				return
			}
		}
	}
	resources, err := resourceClient.List(r.Context(), opts)
	if apierrors.IsNotFound(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to list resources: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnResp(w, resources)
}

// unscopedHandler handles requests for resources in namespaces that don't belong to any project.
func (h *handler) unscopedHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("[%s] handling request %s %s", apiServiceName, r.Method, r.URL.Path)
	vars := mux.Vars(r)
	op := vars["op"]
	var projectsOrNamespaces []string
	if value, ok := vars[projectsOrNamespacesVar]; ok {
		projectsOrNamespaces = strings.Split(value, ",")
	}
	resource, err := h.gvrFromVars(vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	resourceClient, err := h.clientGetter(r, h.restConfig, resource)
	if errors.Is(err, errUnsupportedContentType) {
		logrus.Warnf("[%s] unsupported content type requested: %s", apiServiceName, r.Header.Get("Accept"))
		http.Error(w, "unsupported content type requested", http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get client for config: %v", apiServiceName, err)
		http.Error(w, "failed to get client for config", http.StatusInternalServerError)
	}
	// get namespaces that don't have the projectID label
	selector := labels.NewSelector().Add(*orphanNamespaceRequirement)
	if len(projectsOrNamespaces) > 0 {
		var reqOp selection.Operator
		if op == notOp {
			reqOp = selection.NotIn
		} else {
			reqOp = selection.In
		}
		namespaceRequirement, err := labels.NewRequirement(corev1.LabelMetadataName, reqOp, projectsOrNamespaces)
		if err != nil {
			logrus.Errorf("[%s] failed to create label selector: %v", apiServiceName, err)
			http.Error(w, fmt.Sprintf("failed to create label selector: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		selector = selector.Add(*namespaceRequirement)
	}
	nss, err := h.namespaceCache.List(selector)
	if err != nil {
		logrus.Errorf("[%s] failed to list namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	finalNamespaces := make(map[string]struct{})
	for _, ns := range nss {
		finalNamespaces[ns.Name] = struct{}{}
	}

	resp, err := h.getResourcesForNamespaces(r, resource, resourceClient, finalNamespaces)
	if errors.Is(err, errUnsupportedAction) {
		logrus.Warnf("[%s] %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get resources for namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnResp(w, resp)
}

// scopedHandler handles requests for resources in namespaces in one particular project.
//
// This handler is designed to be compatible with the way Steve distributes and
// aggregates across namespaces that the user has access to. If a user provides
// a project that does not match the one set in /namespaces/{project}, Steve
// may be trying that endpoint separately, so we have to ignore it here.
// Similarly, if the user provides a namespace that is not part of the project
// in /namespaces/{project}, it may be part of another project the user can
// access, so ignore it here.
func (h *handler) scopedHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("[%s] handling request %s %s", apiServiceName, r.Method, r.URL.Path)
	vars := mux.Vars(r)
	project := vars["project"]
	op := vars["op"]
	var projectsOrNamespaces []string
	if _, ok := vars[projectsOrNamespacesVar]; ok {
		projectsOrNamespaces = strings.Split(vars[projectsOrNamespacesVar], ",")
	}
	var projectsList []string
	var namespacesMap map[string]*corev1.Namespace
	resource, err := h.gvrFromVars(vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	resourceClient, err := h.clientGetter(r, h.restConfig, resource)
	if errors.Is(err, errUnsupportedContentType) {
		logrus.Warnf("[%s] unsupported content type requested: %s", apiServiceName, r.Header.Get("Accept"))
		http.Error(w, "unsupported content type requested", http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get client for config: %v", apiServiceName, err)
		http.Error(w, "failed to get client for config", http.StatusInternalServerError)
	}
	if len(projectsOrNamespaces) > 0 {
		projectsList, namespacesMap, err = h.projectsAndNamespaces(projectsOrNamespaces)
		if err != nil {
			logrus.Errorf("[%s] failed to parse projects and namespaces: %v", apiServiceName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// Only include namespaces that are in the project
	// Even if op is "!", excluded namespaces should match the project, since namespaces outside the project are ignored.
	for name, n := range namespacesMap {
		label := n.GetLabels()[nslabels.ProjectIDFieldLabel]
		if label != project {
			delete(namespacesMap, name)
		}
	}
	if op != notOp {
		// /namespaces/{project}/ is specified, so the projects and/or namespaces in the query must match that project
		if len(projectsList) != 0 {
			found := false
			for _, p := range projectsList {
				if p == project {
					projectsList = []string{p} // operator is = and the project matches, so we'll include all namespaces in this project
					found = true
					break
				}
			}
			if !found {
				projectsList = []string{} // operator is = but no project matched, ignore projects and use namespaces to filter
			}
		}
		if len(projectsList) == 0 && len(namespacesMap) == 0 && len(projectsOrNamespaces) > 0 { // selectors were provided but none applied to this project
			logrus.Tracef("[%s] fieldSelector does match project or namespace [%s], skipping", apiServiceName, project)
			h.returnEmpty(r.Context(), w, resource, resourceClient)
			return
		}
	} else {
		if len(projectsList) != 0 {
			for _, p := range projectsList {
				// /namespaces/{project} is specified, but that project is excluded from the query, so return empty
				if p == project {
					logrus.Tracef("[%s] fieldSelector does match project or namespace [%s], skipping", apiServiceName, project)
					h.returnEmpty(r.Context(), w, resource, resourceClient)
					return
				}
			}
			projectsList = []string{} // operator is != but none of the projects in the selector applied, so ignore projects and use namespaces to filter
		}
	}
	selector := labels.Set{nslabels.ProjectIDFieldLabel: project}.AsSelector()
	namespacesList := mapToKeysSlice(namespacesMap)
	if len(namespacesList) != 0 && len(projectsList) == 0 {
		var reqOp selection.Operator
		if op == notOp {
			reqOp = selection.NotIn
		} else {
			reqOp = selection.In
		}
		namespaceRequirement, err := labels.NewRequirement(corev1.LabelMetadataName, reqOp, namespacesList)
		if err != nil {
			logrus.Errorf("[%s] failed to create label selector: %v", apiServiceName, err)
			http.Error(w, fmt.Sprintf("failed to create label selector: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		selector = selector.Add(*namespaceRequirement)
	}
	nss, err := h.namespaceCache.List(selector)
	if err != nil {
		logrus.Errorf("[%s] failed to list namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	finalNamespaces := make(map[string]struct{})
	for _, n := range nss {
		finalNamespaces[n.Name] = struct{}{}
	}
	resp, err := h.getResourcesForNamespaces(r, resource, resourceClient, finalNamespaces)
	if errors.Is(err, errUnsupportedAction) {
		logrus.Warnf("[%s] %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get resources for namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnResp(w, resp)
}

// globalHandler handles admin requests for resources in any projects or any namespaces.
func (h *handler) globalHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("[%s] handling request %s %s", apiServiceName, r.Method, r.URL.Path)
	vars := mux.Vars(r)
	op := vars["op"]
	var projectsOrNamespaces []string
	if value, ok := vars[projectsOrNamespacesVar]; ok {
		projectsOrNamespaces = strings.Split(value, ",")
	}
	resource, err := h.gvrFromVars(vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	resourceClient, err := h.clientGetter(r, h.restConfig, resource)
	if errors.Is(err, errUnsupportedContentType) {
		logrus.Warnf("[%s] unsupported content type requested: %s", apiServiceName, r.Header.Get("Accept"))
		http.Error(w, "unsupported content type requested", http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get client for config: %v", apiServiceName, err)
		http.Error(w, "failed to get client for config", http.StatusInternalServerError)
	}
	projectsList, namespacesMap, err := h.projectsAndNamespaces(projectsOrNamespaces)
	if err != nil {
		logrus.Errorf("[%s] failed to parse projects and namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	namespacesList := mapToKeysSlice(namespacesMap)
	finalNamespaces := make(map[string]struct{})
	// get all namespaces for projects
	selector := labels.NewSelector()
	var projectRequirement, namespaceRequirement *labels.Requirement
	if op == notOp {
		if len(projectsList) > 0 {
			projectRequirement, err = labels.NewRequirement(nslabels.ProjectIDFieldLabel, selection.NotIn, projectsList)
			if err != nil {
				logrus.Errorf("[%s] failed to create label selector: %v", apiServiceName, err)
				http.Error(w, fmt.Sprintf("failed to create label selector: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}
		if len(namespacesList) > 0 {
			namespaceRequirement, err = labels.NewRequirement(corev1.LabelMetadataName, selection.NotIn, namespacesList)
			if err != nil {
				logrus.Errorf("[%s] failed to create label selector: %v", apiServiceName, err)
				http.Error(w, fmt.Sprintf("failed to create label selector: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}
		if projectRequirement != nil {
			selector = selector.Add(*projectRequirement)
		}
		if namespaceRequirement != nil {
			selector = selector.Add(*namespaceRequirement)
		}
	} else {
		if len(projectsList) > 0 {
			projectRequirement, err = labels.NewRequirement(nslabels.ProjectIDFieldLabel, selection.In, projectsList)
			if err != nil {
				logrus.Errorf("[%s] failed to create label selector: %v", apiServiceName, err)
				http.Error(w, fmt.Sprintf("failed to create label selector: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}
		if projectRequirement != nil {
			selector = selector.Add(*projectRequirement)
		}
		for k := range namespacesMap {
			finalNamespaces[k] = struct{}{}
		}
	}
	if !(selector.Empty() && len(finalNamespaces) > 0) { // don't list all namespaces in all projects when we wanted to select by only namespaces
		nss, err := h.namespaceCache.List(selector)
		if err != nil {
			logrus.Errorf("[%s] failed to list namespaces: %v", apiServiceName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, n := range nss {
			finalNamespaces[n.Name] = struct{}{}
		}
	}
	resp, err := h.getResourcesForNamespaces(r, resource, resourceClient, finalNamespaces)
	if errors.Is(err, errUnsupportedAction) {
		logrus.Warnf("[%s] %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		logrus.Errorf("[%s] failed to get resources for namespaces: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnResp(w, resp)
}

// getResourcesForNamespaces concurrently retrieves resources of a given GVR for a slice of namespaces.
func (h *handler) getResourcesForNamespaces(r *http.Request, resource schema.GroupVersionResource, resourceClient dynamic.NamespaceableResourceInterface, namespaces map[string]struct{}) (*unstructured.UnstructuredList, error) {
	itemsChan := make(chan unstructured.Unstructured)
	rowsChan := make(chan interface{})
	itemsList := make([]unstructured.Unstructured, 0)
	rowList := make([]interface{}, 0)
	latestResourceVersion := 0
	resourceVersions := make(chan int)
	var columnDefinitions []interface{}
	eg, ctx := errgroup.WithContext(r.Context())

	opts := metav1.ListOptions{}
	cleanQuery(r)
	err := paramCodec.DecodeParameters(r.URL.Query(), metav1.SchemeGroupVersion, &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query %s: %w", r.URL.Query(), err)
	}
	if opts.Watch {
		return nil, errUnsupportedAction
	}

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		for i := range itemsChan {
			itemsList = append(itemsList, i)
		}
		wg.Done()
	}()

	go func() {
		for r := range rowsChan {
			rowList = append(rowList, r)
		}
		wg.Done()
	}()

	go func() {
		for r := range resourceVersions {
			if r > latestResourceVersion {
				latestResourceVersion = r
			}
		}
		wg.Done()
	}()

	sem := semaphore.NewWeighted(workers)
	for ns := range namespaces {
		ns := ns
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
		}
		eg.Go(func() error {
			defer sem.Release(1)
			if !h.hasAccess(r, ns, resource) {
				return nil
			}
			resourcesForNamespace, err := resourceClient.Namespace(ns).List(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to list resources %s for namespace %s: %w", resource.Resource, ns, err)
			}
			if resourcesForNamespace == nil {
				return nil
			}
			isTable := false
			columnDefinitions, isTable = resourcesForNamespace.Object["columnDefinitions"].([]interface{})
			rows, _ := resourcesForNamespace.Object["rows"].([]interface{})
			if len(resourcesForNamespace.Items) > 0 || (len(rows) > 0) {
				// resourceVersion will be different for every request, and in the end we want the latest one,
				// but we won't know which one is the latest until the channel is done processing.
				rv, err := strconv.Atoi(resourcesForNamespace.GetResourceVersion())
				if err != nil {
					rv = 0
				}
				resourceVersions <- rv
			}
			if isTable {
				for _, r := range rows {
					rowsChan <- r
				}
			}
			if len(resourcesForNamespace.Items) > 0 {
				for _, r := range resourcesForNamespace.Items {
					itemsChan <- r
				}
			}
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	close(itemsChan)
	close(rowsChan)
	close(resourceVersions)
	wg.Wait()
	sortCollection(itemsList)
	sortRows(rowList)
	resourceVersion := strconv.Itoa(latestResourceVersion)
	if resourceVersion == "0" { // this may happen if the namespace slice was empty, but we still need to return a valid resource version.
		resourceVersion, err = emptyResourceVersion(r.Context(), resource, resourceClient)
		if err != nil {
			return nil, err
		}
	}
	if len(columnDefinitions) > 0 {
		return responseTable(resourceVersion, columnDefinitions, rowList), nil
	}
	kind, err := h.apis.GetKindForResource(resource)
	if err != nil {
		return nil, err
	}
	return responseData(resource, kind+"List", resourceVersion, itemsList), nil
}

// gvrFromVars takes a request path like /apps.deployments and gets the registered GVR for APIGroup "apps" and resource "deployments".
func (h *handler) gvrFromVars(vars map[string]string) (schema.GroupVersionResource, error) {
	groupResource := strings.Split(vars["resource"], ".")
	group := ""
	if len(groupResource) > 1 {
		group = strings.Join(groupResource[:len(groupResource)-1], ".")
	}
	resourceName := groupResource[len(groupResource)-1]
	resource, ok := h.apis.Get(resourceName, group)
	if !ok {
		return schema.GroupVersionResource{}, fmt.Errorf("could not find resource %s", vars["resource"])
	}
	return schema.GroupVersionResource{Group: group, Version: resource.Version, Resource: resourceName}, nil
}

// hasAccess checks the auth headers set by the kube API server for the user's access to the requested resource in the requested project.
func (h *handler) hasAccess(r *http.Request, namespace string, resource schema.GroupVersionResource) bool {
	var groups []string
	if g, ok := r.Header["X-Remote-Group"]; ok {
		groups = g
	}
	extras := map[string]authzv1.ExtraValue{}
	prefix := "X-Remote-Extra-"
	for k, v := range r.Header {
		if strings.HasPrefix(k, prefix) {
			extras[k[len(prefix):]] = authzv1.ExtraValue(v)
		}
	}
	review := authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			User:   r.Header.Get("X-Remote-User"),
			Groups: groups,
			Extra:  extras,
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:      "list",
				Resource:  resource.Resource,
				Group:     resource.Group,
				Namespace: namespace,
			},
		},
	}
	result, err := h.sarClient.Create(r.Context(), &review, metav1.CreateOptions{})
	return err == nil && result.Status.Allowed
}

// projectsAndNamespaces sorts a list of projects and namespaces specified by 'fieldSelector=projectsornamespaces=...'
// into separate lists of projects and namespaces.
func (h *handler) projectsAndNamespaces(projectsOrNamespaces []string) ([]string, map[string]*corev1.Namespace, error) {
	projects := []string{}
	namespaces := map[string]*corev1.Namespace{}
	for _, p := range projectsOrNamespaces {
		ns, err := h.namespaceCache.Get(p)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("failed to look up namespace %s in cache: %w", p, err)
		}
		if ns != nil {
			_, ok := ns.GetLabels()[ParentLabel]
			if ok {
				projects = append(projects, ns.Name)
				continue
			}
			_, ok = ns.GetAnnotations()[projectNamespaceAnnotation]
			if ok {
				projects = append(projects, ns.Name)
				continue
			}
			namespaces[ns.Name] = ns
		}
	}
	return projects, namespaces, nil
}

func (h *handler) returnEmpty(ctx context.Context, w http.ResponseWriter, resource schema.GroupVersionResource, resourceClient dynamic.ResourceInterface) {
	rv, err := emptyResourceVersion(ctx, resource, resourceClient)
	if err != nil {
		logrus.Errorf("[%s] error determining resource version: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	kind, err := h.apis.GetKindForResource(resource)
	if err != nil {
		logrus.Errorf("[%s] error parsing resource: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := responseData(resource, kind+"List", rv, []unstructured.Unstructured{})
	returnResp(w, resp)
}

// endpointRestrictions implements negotiation.EndpointRestrictions
type endpointRestrictions struct{}

func (e endpointRestrictions) AllowsMediaTypeTransform(mimeType string, mimeSubType string, gvk *schema.GroupVersionKind) bool {
	if mimeType != "application" {
		return false
	}
	if mimeSubType != "json" {
		return false
	}
	if gvk == nil || gvk.Kind == "" || gvk.Kind == "Table" {
		return true
	}
	return false
}

func (e endpointRestrictions) AllowsServerVersion(string) bool {
	return true
}

func (e endpointRestrictions) AllowsStreamSchema(string) bool {
	return false
}

// clientForRequest sets up a roundtripper for the dynamic client and returns the interface for the given resource.
// The dynamic client ignores the AcceptContentType field on the rest config, so we need to make sure it is set on
// the round tripper in order to retrieve Table-formatted data.
func clientForRequest(r *http.Request, restConfig *rest.Config, resource schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
	acceptedTypes := []runtime.SerializerInfo{
		{
			MediaType:        "application/json",
			MediaTypeType:    "application",
			MediaTypeSubType: "json",
		},
	}
	mediaType, ok := negotiation.NegotiateMediaTypeOptions(r.Header.Get("Accept"), acceptedTypes, endpointRestrictions{})
	if !ok {
		return nil, errUnsupportedContentType
	}
	cfg := rest.CopyConfig(restConfig)
	setOptions := roundTripper(mediaType)
	cfg.Wrap(setOptions)
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return dynamicClient.Resource(resource), nil
}

func sortCollection(resourceCollection []unstructured.Unstructured) {
	sort.Slice(resourceCollection, func(i, j int) bool {
		objI := resourceCollection[i]
		objJ := resourceCollection[j]
		if objI.GetNamespace() < objJ.GetNamespace() {
			return true
		}
		if objI.GetNamespace() > objJ.GetNamespace() {
			return false
		}
		return objI.GetName() < objJ.GetName()
	})
}

func sortRows(rows []interface{}) {
	sort.Slice(rows, func(i, j int) bool {
		rowI, _ := rows[i].(map[string]interface{})
		objI, _ := rowI["object"].(map[string]interface{})
		unstI := unstructured.Unstructured{Object: objI}
		rowJ, _ := rows[j].(map[string]interface{})
		objJ, _ := rowJ["object"].(map[string]interface{})
		unstJ := unstructured.Unstructured{Object: objJ}
		if unstI.GetNamespace() < unstJ.GetNamespace() {
			return true
		}
		if unstI.GetNamespace() > unstJ.GetNamespace() {
			return false
		}
		return unstI.GetName() < unstJ.GetName()
	})
}

func emptyResourceVersion(ctx context.Context, gvr schema.GroupVersionResource, resourceClient dynamic.ResourceInterface) (string, error) {
	// get the list but ignore the resources, this is needed to get the list resource version
	resourceList, err := resourceClient.List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", fmt.Errorf("failed to get resource version for resource %s: %w", gvr.Resource, err)
	}
	return resourceList.GetResourceVersion(), nil
}

func responseData(resource schema.GroupVersionResource, kind, resourceVersion string, items []unstructured.Unstructured) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{
		Items: items,
	}
	list.SetAPIVersion(resource.GroupVersion().String())
	list.SetKind(kind)
	list.SetResourceVersion(resourceVersion)
	return list
}

func responseTable(resourceVersion string, columnDefinitions interface{}, rows interface{}) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetUnstructuredContent(map[string]interface{}{
		"columnDefinitions": columnDefinitions,
		"rows":              rows,
	})
	list.SetAPIVersion("meta.k8s.io/v1")
	list.SetKind("Table")
	list.SetResourceVersion(resourceVersion)
	return list
}

func returnResp(w http.ResponseWriter, resp interface{}) {
	resourceJSON, err := json.Marshal(resp)
	if err != nil {
		logrus.Errorf("[%s] failed to encode response: %v", apiServiceName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resourceJSON)
	return
}

// cleanQuery removes the fieldSelector from the request so that the remainder can be sent on to k8s.
func cleanQuery(req *http.Request) {
	query := req.URL.Query()
	result := url.Values{}
	for k, v := range query {
		if k != queryKey {
			result[k] = v
			continue
		}
		innerResult := []string{}
		for _, q := range v {
			key := queryOpReg.Split(q, 2)[0]
			if key != projectsOrNamespacesKey {
				innerResult = append(innerResult, q)
			}
		}
		if len(innerResult) > 0 {
			result[k] = innerResult
		}
	}
	req.URL.RawQuery = result.Encode()
}

type addOptions struct {
	accept string
	query  map[string]string
	next   http.RoundTripper
}

func (a *addOptions) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Accept", a.accept)
	q := r.URL.Query()
	for k, v := range a.query {
		q.Set(k, v)
	}
	r.URL.RawQuery = q.Encode()
	return a.next.RoundTrip(r)
}

func roundTripper(mediaType negotiation.MediaTypeOptions) func(http.RoundTripper) http.RoundTripper {
	accept := mediaType.Accepted.MediaType
	if mediaType.Convert != nil {
		if mediaType.Convert.Kind != "" {
			accept += ";as=" + mediaType.Convert.Kind
		}
		if mediaType.Convert.Version != "" {
			accept += ";v=" + mediaType.Convert.Version
		}
		if mediaType.Convert.Group != "" {
			accept += ";g=" + mediaType.Convert.Group
		}
	}
	return func(rt http.RoundTripper) http.RoundTripper {
		ao := addOptions{
			accept: accept,
			next:   rt,
		}
		if mediaType.Convert != nil && mediaType.Convert.Kind == "Table" {
			ao.query = map[string]string{
				"includeObject": "Object",
			}
		}
		return &ao
	}
}

func mapToKeysSlice(m map[string]*corev1.Namespace) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}
