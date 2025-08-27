package main

import (
	"crypto"
	"crypto/rand"
	crsa "crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// loggingResponseWriter captures the status code for access logs
type loggingResponseWriter struct {
	http.ResponseWriter
	code int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

type namespaceObj struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Status     map[string]interface{} `json:"status,omitempty"`
}

type stateStore struct {
	mu         sync.RWMutex
	rv         uint64
	namespaces map[string]*namespaceObj
	// namespaced stores
	serviceAccounts map[string]map[string]map[string]interface{} // ns -> name -> SA object
	secrets         map[string]map[string]map[string]interface{} // ns -> name -> Secret object
	// RBAC stores
	clusterRoles        map[string]map[string]interface{}            // name -> ClusterRole
	clusterRoleBindings map[string]map[string]interface{}            // name -> ClusterRoleBinding
	roles               map[string]map[string]map[string]interface{} // ns -> name -> Role
	roleBindings        map[string]map[string]map[string]interface{} // ns -> name -> RoleBinding
}

func newState() *stateStore {
	s := &stateStore{
		namespaces:          map[string]*namespaceObj{},
		rv:                  1,
		serviceAccounts:     map[string]map[string]map[string]interface{}{},
		secrets:             map[string]map[string]map[string]interface{}{},
		clusterRoles:        map[string]map[string]interface{}{},
		clusterRoleBindings: map[string]map[string]interface{}{},
		roles:               map[string]map[string]map[string]interface{}{},
		roleBindings:        map[string]map[string]map[string]interface{}{},
	}
	// seed a few namespaces Rancher expects
	for _, ns := range []string{"kube-system", "cattle-system", "cattle-impersonation-system", "cattle-fleet-system"} {
		s.ensureNS(ns)
	}
	return s
}

func (s *stateStore) nextRV() string { s.rv++; return fmt.Sprintf("%d", s.rv) }

func (s *stateStore) ensureNS(name string) *namespaceObj {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.namespaces[name]; ok {
		return n
	}
	obj := &namespaceObj{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: map[string]interface{}{
			"name":              name,
			"creationTimestamp": time.Now().UTC().Format(time.RFC3339),
			"resourceVersion":   s.nextRV(),
			"uid":               fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			"labels":            map[string]interface{}{},
		},
		Status: map[string]interface{}{"phase": "Active"},
	}
	s.namespaces[name] = obj
	return obj
}

func (s *stateStore) listNS() []*namespaceObj {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*namespaceObj, 0, len(s.namespaces))
	for _, v := range s.namespaces {
		out = append(out, v)
	}
	return out
}

func (s *stateStore) getNS(name string) (*namespaceObj, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.namespaces[name]
	return v, ok
}

// --- ServiceAccount and Secret helpers ---

func (s *stateStore) upsertServiceAccount(ns string, sa map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.serviceAccounts[ns] == nil {
		s.serviceAccounts[ns] = map[string]map[string]interface{}{}
	}
	meta, _ := sa["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		sa["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	// Honor generateName if name is empty
	if name == "" {
		if gn, _ := meta["generateName"].(string); gn != "" {
			// Kubernetes appends a random suffix; emulate with a short hex timestamp
			name = fmt.Sprintf("%s%x", gn, time.Now().UnixNano()&0xfffff)
			meta["name"] = name
		}
	}
	if name == "" {
		return nil
	}
	if meta["namespace"] == nil {
		meta["namespace"] = ns
	}
	if meta["uid"] == nil {
		meta["uid"] = fmt.Sprintf("%s-%s-%d", ns, name, time.Now().UnixNano())
	}
	meta["resourceVersion"] = s.nextRV()
	if sa["apiVersion"] == nil {
		sa["apiVersion"] = "v1"
	}
	if sa["kind"] == nil {
		sa["kind"] = "ServiceAccount"
	}
	s.serviceAccounts[ns][name] = sa
	return sa
}

func (s *stateStore) getServiceAccount(ns, name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.serviceAccounts[ns]; m != nil {
		v, ok := m[name]
		return v, ok
	}
	return nil, false
}

func (s *stateStore) listServiceAccounts(ns string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	if m := s.serviceAccounts[ns]; m != nil {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

// listAllServiceAccounts aggregates all SAs across namespaces
func (s *stateStore) listAllServiceAccounts() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	for _, m := range s.serviceAccounts {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

func (s *stateStore) upsertSecret(ns string, sec map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secrets[ns] == nil {
		s.secrets[ns] = map[string]map[string]interface{}{}
	}
	meta, _ := sec["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		sec["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	if name == "" {
		if gn, _ := meta["generateName"].(string); gn != "" {
			name = fmt.Sprintf("%s%x", gn, time.Now().UnixNano()&0xfffff)
		} else {
			name = fmt.Sprintf("secret-%d", time.Now().UnixNano())
		}
		meta["name"] = name
	}
	if meta["namespace"] == nil {
		meta["namespace"] = ns
	}
	if meta["uid"] == nil {
		meta["uid"] = fmt.Sprintf("%s-%s-%d", ns, name, time.Now().UnixNano())
	}
	// Convert stringData to data (base64-encoded) like Kubernetes does
	if sd, _ := sec["stringData"].(map[string]interface{}); sd != nil {
		data, _ := sec["data"].(map[string]interface{})
		if data == nil {
			data = map[string]interface{}{}
		}
		for k, v := range sd {
			if s, ok := v.(string); ok {
				data[k] = base64.StdEncoding.EncodeToString([]byte(s))
			}
		}
		sec["data"] = data
		delete(sec, "stringData")
	}
	meta["resourceVersion"] = s.nextRV()
	if sec["apiVersion"] == nil {
		sec["apiVersion"] = "v1"
	}
	if sec["kind"] == nil {
		sec["kind"] = "Secret"
	}
	s.secrets[ns][name] = sec
	return sec
}

func (s *stateStore) getSecret(ns, name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.secrets[ns]; m != nil {
		v, ok := m[name]
		return v, ok
	}
	return nil, false
}

func (s *stateStore) listSecrets(ns string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	if m := s.secrets[ns]; m != nil {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

// listAllSecrets aggregates all Secrets across namespaces
func (s *stateStore) listAllSecrets() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	for _, m := range s.secrets {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

// --- RBAC helpers ---
func (s *stateStore) upsertClusterRole(obj map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		obj["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	if name == "" {
		return nil
	}
	meta["resourceVersion"] = s.nextRV()
	if obj["apiVersion"] == nil {
		obj["apiVersion"] = "rbac.authorization.k8s.io/v1"
	}
	if obj["kind"] == nil {
		obj["kind"] = "ClusterRole"
	}
	s.clusterRoles[name] = obj
	return obj
}

func (s *stateStore) getClusterRole(name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.clusterRoles[name]
	return v, ok
}

func (s *stateStore) listClusterRoles() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	for _, v := range s.clusterRoles {
		out = append(out, v)
	}
	return out
}

func (s *stateStore) upsertClusterRoleBinding(obj map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		obj["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	if name == "" {
		return nil
	}
	meta["resourceVersion"] = s.nextRV()
	if obj["apiVersion"] == nil {
		obj["apiVersion"] = "rbac.authorization.k8s.io/v1"
	}
	if obj["kind"] == nil {
		obj["kind"] = "ClusterRoleBinding"
	}
	s.clusterRoleBindings[name] = obj
	return obj
}

func (s *stateStore) getClusterRoleBinding(name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.clusterRoleBindings[name]
	return v, ok
}

func (s *stateStore) listClusterRoleBindings() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	for _, v := range s.clusterRoleBindings {
		out = append(out, v)
	}
	return out
}

// Add delete methods for resources
func (s *stateStore) deleteClusterRoleBinding(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.clusterRoleBindings[name]; exists {
		delete(s.clusterRoleBindings, name)
		return true
	}
	return false
}

func (s *stateStore) deleteRole(ns, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.roles[ns]; m != nil {
		if _, exists := m[name]; exists {
			delete(m, name)
			return true
		}
	}
	return false
}

func (s *stateStore) deleteRoleBinding(ns, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.roleBindings[ns]; m != nil {
		if _, exists := m[name]; exists {
			delete(m, name)
			return true
		}
	}
	return false
}

func (s *stateStore) deleteServiceAccount(ns, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.serviceAccounts[ns]; m != nil {
		if _, exists := m[name]; exists {
			delete(m, name)
			return true
		}
	}
	return false
}

func (s *stateStore) deleteSecret(ns, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.secrets[ns]; m != nil {
		if _, exists := m[name]; exists {
			delete(m, name)
			return true
		}
	}
	return false
}

func (s *stateStore) upsertRole(ns string, obj map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roles[ns] == nil {
		s.roles[ns] = map[string]map[string]interface{}{}
	}
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		obj["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	if name == "" {
		return nil
	}
	meta["namespace"] = ns
	meta["resourceVersion"] = s.nextRV()
	if obj["apiVersion"] == nil {
		obj["apiVersion"] = "rbac.authorization.k8s.io/v1"
	}
	if obj["kind"] == nil {
		obj["kind"] = "Role"
	}
	s.roles[ns][name] = obj
	return obj
}

func (s *stateStore) getRole(ns, name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.roles[ns]; m != nil {
		v, ok := m[name]
		return v, ok
	}
	return nil, false
}

func (s *stateStore) listRoles(ns string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	if m := s.roles[ns]; m != nil {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

func (s *stateStore) upsertRoleBinding(ns string, obj map[string]interface{}) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roleBindings[ns] == nil {
		s.roleBindings[ns] = map[string]map[string]interface{}{}
	}
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		obj["metadata"] = meta
	}
	name, _ := meta["name"].(string)
	if name == "" {
		return nil
	}
	meta["namespace"] = ns
	meta["resourceVersion"] = s.nextRV()
	if obj["apiVersion"] == nil {
		obj["apiVersion"] = "rbac.authorization.k8s.io/v1"
	}
	if obj["kind"] == nil {
		obj["kind"] = "RoleBinding"
	}
	s.roleBindings[ns][name] = obj
	return obj
}

func (s *stateStore) getRoleBinding(ns, name string) (map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.roleBindings[ns]; m != nil {
		v, ok := m[name]
		return v, ok
	}
	return nil, false
}

func (s *stateStore) listRoleBindings(ns string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []map[string]interface{}{}
	if m := s.roleBindings[ns]; m != nil {
		for _, v := range m {
			out = append(out, v)
		}
	}
	return out
}

func (s *stateStore) ensureTokenSecretForSA(ns, saName string) map[string]interface{} {
	sa, ok := s.getServiceAccount(ns, saName)
	if !ok {
		return nil
	}
	meta, _ := sa["metadata"].(map[string]interface{})
	if meta == nil {
		return nil
	}
	saUID, _ := meta["uid"].(string)
	// Create a deterministic name to allow Rancher to GET by name conventionally
	secName := fmt.Sprintf("%s-token-%x", saName, time.Now().Unix()%0xffff)
	annotations := map[string]interface{}{
		"kubernetes.io/service-account.name": saName,
		"kubernetes.io/service-account.uid":  saUID,
	}
	// Mint a short-lived RS256 JWT for the SA and base64-encode Secret data as Kubernetes does
	tok := mintServiceAccountJWT(ns, saName, saUID, secName, nil, 3600)
	data := map[string]interface{}{}
	data["token"] = base64.StdEncoding.EncodeToString([]byte(tok))
	data["namespace"] = base64.StdEncoding.EncodeToString([]byte(ns))
	if len(serverCACertPEM) > 0 {
		data["ca.crt"] = base64.StdEncoding.EncodeToString(serverCACertPEM)
	} else {
		data["ca.crt"] = base64.StdEncoding.EncodeToString([]byte("FAKECERT"))
	}
	ownerRefs := []map[string]interface{}{
		{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"name":       saName,
			"uid":        saUID,
		},
	}
	sec := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":            secName,
			"namespace":       ns,
			"annotations":     annotations,
			"ownerReferences": ownerRefs,
		},
		"type": "kubernetes.io/service-account-token",
		"data": data,
	}
	created := s.upsertSecret(ns, sec)
	// Also add a reference to SA.secrets
	s.addSecretRefToSA(ns, saName, secName)
	return created
}

// populateSecretAsSAToken ensures the named Secret in ns is of type service-account-token and contains
// token data and annotations for the provided ServiceAccount. It returns the updated Secret.
func (s *stateStore) populateSecretAsSAToken(ns, secName, saName string) map[string]interface{} {
	sa, ok := s.getServiceAccount(ns, saName)
	if !ok {
		return nil
	}
	md, _ := sa["metadata"].(map[string]interface{})
	saUID, _ := md["uid"].(string)
	// Mint token bound to this secret name
	tok := mintServiceAccountJWT(ns, saName, saUID, secName, nil, 3600)
	data := map[string]interface{}{
		"token":     base64.StdEncoding.EncodeToString([]byte(tok)),
		"namespace": base64.StdEncoding.EncodeToString([]byte(ns)),
	}
	if len(serverCACertPEM) > 0 {
		data["ca.crt"] = base64.StdEncoding.EncodeToString(serverCACertPEM)
	} else {
		data["ca.crt"] = base64.StdEncoding.EncodeToString([]byte("FAKECERT"))
	}
	annotations := map[string]interface{}{
		"kubernetes.io/service-account.name": saName,
		"kubernetes.io/service-account.uid":  saUID,
	}
	// Fetch existing secret if present, else create a stub
	sec, _ := s.getSecret(ns, secName)
	if sec == nil {
		sec = map[string]interface{}{}
	}
	if sec["apiVersion"] == nil {
		sec["apiVersion"] = "v1"
	}
	if sec["kind"] == nil {
		sec["kind"] = "Secret"
	}
	smd, _ := sec["metadata"].(map[string]interface{})
	if smd == nil {
		smd = map[string]interface{}{}
		sec["metadata"] = smd
	}
	smd["name"] = secName
	smd["namespace"] = ns
	// Owner ref
	ownerRefs := []map[string]interface{}{{"apiVersion": "v1", "kind": "ServiceAccount", "name": saName, "uid": saUID}}
	sec["type"] = "kubernetes.io/service-account-token"
	// Merge annotations
	if existAnn, _ := smd["annotations"].(map[string]interface{}); existAnn != nil {
		for k, v := range annotations {
			existAnn[k] = v
		}
		smd["annotations"] = existAnn
	} else {
		smd["annotations"] = annotations
	}
	sec["data"] = data
	sec["metadata"] = smd
	sec["ownerReferences"] = ownerRefs
	updated := s.upsertSecret(ns, sec)
	s.addSecretRefToSA(ns, saName, secName)
	return updated
}

// addSecretRefToSA appends a named secret reference into the SA's secrets field and bumps RV
func (s *stateStore) addSecretRefToSA(ns, saName, secName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sam := s.serviceAccounts[ns]
	if sam == nil {
		return
	}
	sa := sam[saName]
	if sa == nil {
		return
	}
	// secrets: [{name: secName}]
	arr, _ := sa["secrets"].([]interface{})
	// check if exists
	for _, it := range arr {
		if m, ok := it.(map[string]interface{}); ok {
			if m["name"] == secName {
				return
			}
		}
	}
	arr = append(arr, map[string]interface{}{"name": secName})
	sa["secrets"] = arr
	// bump rv
	if meta, _ := sa["metadata"].(map[string]interface{}); meta != nil {
		meta["resourceVersion"] = s.nextRV()
	}
	// write back
	sam[saName] = sa
}

// upsertNS applies a full-object update. Accept updates even when resourceVersion is stale by merging and bumping rv.
func (s *stateStore) upsertNS(in *namespaceObj) *namespaceObj {
	if in == nil {
		return nil
	}
	name, _ := in.Metadata["name"].(string)
	if name == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.namespaces[name]
	newRV := s.nextRV()
	if !ok {
		if in.Metadata == nil {
			in.Metadata = map[string]interface{}{}
		}
		in.Metadata["resourceVersion"] = newRV
		if in.Status == nil {
			in.Status = map[string]interface{}{"phase": "Active"}
		}
		s.namespaces[name] = in
		return in
	}
	if in.Metadata == nil {
		in.Metadata = map[string]interface{}{}
	}
	// Merge labels
	curLabels, _ := cur.Metadata["labels"].(map[string]interface{})
	inLabels, _ := in.Metadata["labels"].(map[string]interface{})
	if curLabels == nil {
		curLabels = map[string]interface{}{}
	}
	for k, v := range inLabels {
		curLabels[k] = v
	}
	updated := &namespaceObj{
		APIVersion: cur.APIVersion,
		Kind:       cur.Kind,
		Metadata: map[string]interface{}{
			"name":              name,
			"creationTimestamp": cur.Metadata["creationTimestamp"],
			"uid":               cur.Metadata["uid"],
			"labels":            curLabels,
			"resourceVersion":   newRV,
		},
		Status: map[string]interface{}{"phase": "Active"},
	}
	s.namespaces[name] = updated
	return updated
}

// isWatch returns true if the request has watch=true
func isWatch(r *http.Request) bool {
	q := r.URL.Query()
	return strings.ToLower(q.Get("watch")) == "true"
}

// wantBookmark returns true if client opted-in via allowWatchBookmarks=true
func wantBookmark(r *http.Request) bool {
	q := r.URL.Query()
	v := strings.ToLower(q.Get("allowWatchBookmarks"))
	return v == "true" || v == "1" || v == "yes"
}

// writeWatchEvents writes a minimal stream of WatchEvents for given objects.
// Each event is {"type":"ADDED","object":obj}. Optionally appends a BOOKMARK with the correct kind.
func writeWatchEvents(w http.ResponseWriter, objs []map[string]interface{}, groupVersion, kind, resourceVersion string, includeBookmark bool) {
	// Match Kubernetes more closely
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Best effort streaming
	if flusher, ok := w.(http.Flusher); ok {
		enc := json.NewEncoder(w)
		wrote := false
		for _, obj := range objs {
			_ = enc.Encode(map[string]interface{}{"type": "ADDED", "object": obj})
			flusher.Flush()
			wrote = true
		}
		// If client didn't request bookmark but we otherwise wrote nothing, emit one anyway
		if !wrote {
			includeBookmark = true
			if resourceVersion == "" {
				resourceVersion = "1"
			}
		}
		if includeBookmark && resourceVersion != "" {
			// Emit a minimal bookmark using PartialObjectMetadata as Kubernetes does
			_ = enc.Encode(map[string]interface{}{
				"type": "BOOKMARK",
				"object": map[string]interface{}{
					"kind":       "PartialObjectMetadata",
					"apiVersion": "meta.k8s.io/v1",
					"metadata":   map[string]interface{}{"resourceVersion": resourceVersion},
				},
			})
			flusher.Flush()
		}
		return
	}
	// Fallback non-streaming
	enc := json.NewEncoder(w)
	wrote := false
	for _, obj := range objs {
		_ = enc.Encode(map[string]interface{}{"type": "ADDED", "object": obj})
		wrote = true
	}
	if !wrote {
		if resourceVersion == "" {
			resourceVersion = "1"
		}
		_ = enc.Encode(map[string]interface{}{
			"type": "BOOKMARK",
			"object": map[string]interface{}{
				"kind":       "PartialObjectMetadata",
				"apiVersion": "meta.k8s.io/v1",
				"metadata":   map[string]interface{}{"resourceVersion": resourceVersion},
			},
		})
	}
}

// k8sStatus constructs a minimal Kubernetes Status object for errors
func k8sStatus(code int, reason, message string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Failure",
		"message":    message,
		"reason":     reason,
		"code":       code,
	}
}

var (
	// Signing key for service account tokens minted by this simulator
	saSigningKey *crsa.PrivateKey
	// PEM of the self-signed cert served by this API (used as fake CA for Secret data)
	serverCACertPEM []byte
)

func selfSignedCert(host string) (tls.Certificate, []byte, error) {
	priv, err := crsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: host, Organization: []string{"simulated-apiserver"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost", host},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	return cert, certPEM, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// mintServiceAccountJWT creates a signed RS256 JWT resembling a Kubernetes SA token
func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mintServiceAccountJWT(ns, saName, saUID, secName string, audience []string, expiresInSeconds int64) string {
	if len(audience) == 0 {
		audience = []string{"https://kubernetes.default.svc"}
	}
	if expiresInSeconds <= 0 {
		expiresInSeconds = 3600
	}
	now := time.Now()
	// Header
	hdr := map[string]interface{}{"alg": "RS256", "typ": "JWT"}
	hdrJSON, _ := json.Marshal(hdr)
	// Claims
	claims := map[string]interface{}{
		"iss":                                    "kubernetes/serviceaccount",
		"sub":                                    fmt.Sprintf("system:serviceaccount:%s:%s", ns, saName),
		"kubernetes.io/serviceaccount/namespace": ns,
		"kubernetes.io/serviceaccount/secret.name":          secName,
		"kubernetes.io/serviceaccount/service-account.name": saName,
		"kubernetes.io/serviceaccount/service-account.uid":  saUID,
		"aud": audience,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(time.Duration(expiresInSeconds) * time.Second).Unix(),
	}
	payloadJSON, _ := json.Marshal(claims)
	unsigned := b64url(hdrJSON) + "." + b64url(payloadJSON)
	if saSigningKey == nil {
		if key, err := crsa.GenerateKey(rand.Reader, 2048); err == nil {
			saSigningKey = key
		}
	}
	if saSigningKey == nil {
		return fmt.Sprintf("opaque-%d", time.Now().UnixNano())
	}
	h := sha256.New()
	_, _ = h.Write([]byte(unsigned))
	sum := h.Sum(nil)
	sig, err := crsa.SignPKCS1v15(rand.Reader, saSigningKey, crypto.SHA256, sum)
	if err != nil {
		// fallback opaque
		return fmt.Sprintf("opaque-%d", time.Now().UnixNano())
	}
	return unsigned + "." + b64url(sig)
}

func main() {
	var port int
	var dbPath string
	flag.IntVar(&port, "port", 0, "listen port")
	flag.StringVar(&dbPath, "db", "", "database path (optional)")
	flag.Parse()
	if port == 0 {
		log.Fatal("--port is required")
	}
	if dbPath != "" {
		_ = os.MkdirAll(path.Dir(dbPath), 0755)
		f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_RDWR, 0644)
		if err == nil {
			_ = f.Close()
		}
	}

	// Derive cluster name from --db path to place access logs in the per-cluster folder
	// Expected format: clusters/<clusterName>.db
	clusterName := ""
	if dbPath != "" {
		base := path.Base(dbPath)
		// Trim extension .db if present
		ext := path.Ext(base)
		clusterName = strings.TrimSuffix(base, ext)
	}
	var accessLogFile *os.File
	if clusterName != "" {
		if home, err := os.UserHomeDir(); err == nil {
			clusterDir := filepath.Join(home, ".kwok", "clusters", clusterName)
			// best-effort ensure dir exists
			_ = os.MkdirAll(clusterDir, 0755)
			// Open sim-access.log in append mode
			if f, err := os.OpenFile(filepath.Join(clusterDir, "sim-access.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				accessLogFile = f
				defer accessLogFile.Close()
			}
		}
	}

	// Helper to write access logs to stderr and per-cluster file (if available)
	logAccessf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Print(msg)
		if accessLogFile != nil {
			// Prepend RFC3339 timestamp for readability
			_, _ = accessLogFile.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n")
		}
	}

	st := newState()

	// Initialize SA signing key once
	if k, err := crsa.GenerateKey(rand.Reader, 2048); err == nil {
		saSigningKey = k
	}

	mux := http.NewServeMux()
	// Health
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	// Ready
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })

	// Discovery: /version
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"major":      "1",
			"minor":      "28",
			"gitVersion": "v1.28.1+k3s1",
			"goVersion":  runtime.Version(),
			"platform":   runtime.GOOS + "/" + runtime.GOARCH,
		})
	})
	// Discovery: /api
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIVersions", "versions": []string{"v1"}, "serverAddressByClientCIDRs": []interface{}{}})
	})
	// Discovery: /apis group list
	mux.HandleFunc("/apis", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"kind":       "APIGroupList",
			"apiVersion": "v1",
			"groups": []map[string]interface{}{
				{"name": "rbac.authorization.k8s.io", "versions": []map[string]interface{}{{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}},
				{"name": "apiregistration.k8s.io", "versions": []map[string]interface{}{{"groupVersion": "apiregistration.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "apiregistration.k8s.io/v1", "version": "v1"}},
				{"name": "authentication.k8s.io", "versions": []map[string]interface{}{{"groupVersion": "authentication.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "authentication.k8s.io/v1", "version": "v1"}},
			},
		})
	})
	// Discovery: generic /apis/{group}/{version} to avoid 404s during probing
	mux.HandleFunc("/apis/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/apis/")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": parts[0] + "/" + parts[1], "resources": []interface{}{}})
			return
		}
		http.NotFound(w, r)
	})
	// Discovery: /apis/rbac.authorization.k8s.io/v1
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "rbac.authorization.k8s.io/v1", "resources": []map[string]interface{}{{"name": "clusterroles", "namespaced": false, "kind": "ClusterRole"}, {"name": "clusterrolebindings", "namespaced": false, "kind": "ClusterRoleBinding"}, {"name": "roles", "namespaced": true, "kind": "Role"}, {"name": "rolebindings", "namespaced": true, "kind": "RoleBinding"}}})
	})
	// Discovery: /apis/apiregistration.k8s.io/v1
	mux.HandleFunc("/apis/apiregistration.k8s.io/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "apiregistration.k8s.io/v1", "resources": []map[string]interface{}{{"name": "apiservices", "namespaced": false, "kind": "APIService"}}})
	})

	// RBAC cluster-scoped lists/watch and CRUD
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterroles", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			items := []map[string]interface{}{}
			rv := ""
			for _, it := range st.listClusterRoles() {
				items = append(items, it)
				if md, _ := it["metadata"].(map[string]interface{}); md != nil {
					if v, _ := md["resourceVersion"].(string); v != "" {
						rv = v
					}
				}
			}
			writeWatchEvents(w, items, "rbac.authorization.k8s.io/v1", "ClusterRole", rv, wantBookmark(r))
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := []interface{}{}
			for _, it := range st.listClusterRoles() {
				items = append(items, it)
			}
			writeJSON(w, 200, map[string]interface{}{"kind": "ClusterRoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
			return
		case http.MethodPost:
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			out := st.upsertClusterRole(in)
			writeJSON(w, 201, out)
			return
		case http.MethodDelete:
			// Handle DELETE for cluster roles collection (not supported in real K8s)
			writeJSON(w, 405, map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Status",
				"status":     "Failure",
				"message":    "Method not allowed",
				"reason":     "MethodNotAllowed",
				"code":       405,
			})
			return
		default:
			w.WriteHeader(405)
			return
		}
	})
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterroles/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/clusterroles/")
		if name == "" {
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if obj, ok := st.getClusterRole(name); ok {
				writeJSON(w, 200, obj)
				return
			}
			writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("clusterroles \"%s\" not found", name)))
			return
		case http.MethodPut, http.MethodPatch:
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in == nil {
				in = map[string]interface{}{}
			}
			md, _ := in["metadata"].(map[string]interface{})
			if md == nil {
				md = map[string]interface{}{}
				in["metadata"] = md
			}
			md["name"] = name
			out := st.upsertClusterRole(in)
			writeJSON(w, 200, out)
			return
		case http.MethodDelete:
			// Handle DELETE for individual cluster roles
			writeJSON(w, 200, map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Status",
				"status":     "Success",
				"message":    "ClusterRole deleted successfully",
			})
			return
		default:
			w.WriteHeader(405)
			return
		}
	})
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			items := []map[string]interface{}{}
			rv := ""
			for _, it := range st.listClusterRoleBindings() {
				items = append(items, it)
				if md, _ := it["metadata"].(map[string]interface{}); md != nil {
					if v, _ := md["resourceVersion"].(string); v != "" {
						rv = v
					}
				}
			}
			writeWatchEvents(w, items, "rbac.authorization.k8s.io/v1", "ClusterRoleBinding", rv, wantBookmark(r))
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := []interface{}{}
			for _, it := range st.listClusterRoleBindings() {
				items = append(items, it)
			}
			writeJSON(w, 200, map[string]interface{}{"kind": "ClusterRoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items})
			return
		case http.MethodPost:
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			out := st.upsertClusterRoleBinding(in)
			writeJSON(w, 201, out)
			return
		case http.MethodDelete:
			// Handle DELETE for cluster role bindings collection (not supported in real K8s)
			writeJSON(w, 405, map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Status",
				"status":     "Failure",
				"message":    "Method not allowed",
				"reason":     "MethodNotAllowed",
				"code":       405,
			})
			return
		default:
			w.WriteHeader(405)
			return
		}
	})
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/")
		if name == "" {
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if obj, ok := st.getClusterRoleBinding(name); ok {
				writeJSON(w, 200, obj)
				return
			}
			writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("clusterrolebindings \"%s\" not found", name)))
			return
		case http.MethodPut, http.MethodPatch:
			var in map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in == nil {
				in = map[string]interface{}{}
			}
			md, _ := in["metadata"].(map[string]interface{})
			if md == nil {
				md = map[string]interface{}{}
				in["metadata"] = md
			}
			md["name"] = name
			out := st.upsertClusterRoleBinding(in)
			writeJSON(w, 200, out)
			return
		case http.MethodDelete:
			// Handle DELETE for individual cluster role bindings
			if st.deleteClusterRoleBinding(name) {
				writeJSON(w, 200, map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Status",
					"status":     "Success",
					"message":    "ClusterRoleBinding deleted successfully",
				})
			} else {
				writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("clusterrolebindings \"%s\" not found", name)))
			}
			return
		default:
			w.WriteHeader(405)
			return
		}
	})
	// RBAC namespaced resources list/watch
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1/namespaces/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/apis/rbac.authorization.k8s.io/v1/namespaces/")
		parts := strings.Split(rest, "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		ns := parts[0]
		res := parts[1]
		switch res {
		case "roles":
			if isWatch(r) {
				items := []map[string]interface{}{}
				rv := ""
				for _, it := range st.listRoles(ns) {
					items = append(items, it)
					if md, _ := it["metadata"].(map[string]interface{}); md != nil {
						if v, _ := md["resourceVersion"].(string); v != "" {
							rv = v
						}
					}
				}
				writeWatchEvents(w, items, "rbac.authorization.k8s.io/v1", "Role", rv, wantBookmark(r))
				return
			}
			switch r.Method {
			case http.MethodGet:
				items := []interface{}{}
				for _, it := range st.listRoles(ns) {
					items = append(items, it)
				}
				writeJSON(w, 200, map[string]interface{}{"kind": "RoleList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items, "metadata": map[string]interface{}{"namespace": ns}})
				return
			case http.MethodPost:
				var in map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&in)
				out := st.upsertRole(ns, in)
				writeJSON(w, 201, out)
				return
			default:
				w.WriteHeader(405)
				return
			}
		case "rolebindings":
			if isWatch(r) {
				items := []map[string]interface{}{}
				rv := ""
				for _, it := range st.listRoleBindings(ns) {
					items = append(items, it)
					if md, _ := it["metadata"].(map[string]interface{}); md != nil {
						if v, _ := md["resourceVersion"].(string); v != "" {
							rv = v
						}
					}
				}
				writeWatchEvents(w, items, "rbac.authorization.k8s.io/v1", "RoleBinding", rv, wantBookmark(r))
				return
			}
			switch r.Method {
			case http.MethodGet:
				items := []interface{}{}
				for _, it := range st.listRoleBindings(ns) {
					items = append(items, it)
				}
				writeJSON(w, 200, map[string]interface{}{"kind": "RoleBindingList", "apiVersion": "rbac.authorization.k8s.io/v1", "items": items, "metadata": map[string]interface{}{"namespace": ns}})
				return
			case http.MethodPost:
				var in map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&in)
				out := st.upsertRoleBinding(ns, in)
				writeJSON(w, 201, out)
				return
			default:
				w.WriteHeader(405)
				return
			}
		case "roles/":
			// not used; handled above via namespaced collection
			http.NotFound(w, r)
			return
		case "rolebindings/":
			http.NotFound(w, r)
			return
		default:
			http.NotFound(w, r)
			return
		}
	})
	// APIService list/watch
	mux.HandleFunc("/apis/apiregistration.k8s.io/v1/apiservices", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchEvents(w, nil, "apiregistration.k8s.io/v1", "APIService", "1", wantBookmark(r))
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "APIServiceList", "apiVersion": "apiregistration.k8s.io/v1", "items": []interface{}{}})
	})

	// Discovery: /api/v1
	mux.HandleFunc("/api/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "v1", "resources": []map[string]interface{}{
			{"name": "namespaces", "namespaced": false, "kind": "Namespace"},
			{"name": "nodes", "namespaced": false, "kind": "Node"},
			{"name": "serviceaccounts", "namespaced": true, "kind": "ServiceAccount"},
			{"name": "secrets", "namespaced": true, "kind": "Secret"},
			{"name": "resourcequotas", "namespaced": true, "kind": "ResourceQuota"},
			{"name": "limitranges", "namespaced": true, "kind": "LimitRange"},
		}})
	})

	// Cluster-scoped aggregated lists for namespaced resources (match Kubernetes behavior)
	// Secrets across all namespaces
	mux.HandleFunc("/api/v1/secrets", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			items := []map[string]interface{}{}
			rv := ""
			for _, it := range st.listAllSecrets() {
				items = append(items, it)
				if md, _ := it["metadata"].(map[string]interface{}); md != nil {
					if v, _ := md["resourceVersion"].(string); v != "" {
						rv = v
					}
				}
			}
			writeWatchEvents(w, items, "v1", "Secret", rv, wantBookmark(r))
			return
		}
		items := []interface{}{}
		for _, it := range st.listAllSecrets() {
			items = append(items, it)
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "SecretList", "apiVersion": "v1", "items": items})
	})

	// ServiceAccounts across all namespaces
	mux.HandleFunc("/api/v1/serviceaccounts", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			items := []map[string]interface{}{}
			rv := ""
			for _, it := range st.listAllServiceAccounts() {
				items = append(items, it)
				if md, _ := it["metadata"].(map[string]interface{}); md != nil {
					if v, _ := md["resourceVersion"].(string); v != "" {
						rv = v
					}
				}
			}
			writeWatchEvents(w, items, "v1", "ServiceAccount", rv, wantBookmark(r))
			return
		}
		items := []interface{}{}
		for _, it := range st.listAllServiceAccounts() {
			items = append(items, it)
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "ServiceAccountList", "apiVersion": "v1", "items": items})
	})

	// ResourceQuotas across all namespaces (empty list ok)
	mux.HandleFunc("/api/v1/resourcequotas", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchEvents(w, nil, "v1", "ResourceQuota", "1", wantBookmark(r))
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "ResourceQuotaList", "apiVersion": "v1", "items": []interface{}{}})
	})

	// LimitRanges across all namespaces (empty list ok)
	mux.HandleFunc("/api/v1/limitranges", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			writeWatchEvents(w, nil, "v1", "LimitRange", "1", wantBookmark(r))
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "LimitRangeList", "apiVersion": "v1", "items": []interface{}{}})
	})

	// TokenRequest subresource: /apis/authentication.k8s.io/v1/namespaces/{ns}/serviceaccounts/{name}/token
	mux.HandleFunc("/apis/authentication.k8s.io/v1/namespaces/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/apis/authentication.k8s.io/v1/namespaces/")
		parts := strings.Split(rest, "/")
		if len(parts) < 4 {
			http.NotFound(w, r)
			return
		}
		ns := parts[0]
		if parts[1] != "serviceaccounts" {
			http.NotFound(w, r)
			return
		}
		saName := parts[2]
		if parts[3] != "token" {
			http.NotFound(w, r)
			return
		}
		// Parse a minimal TokenRequest
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		spec, _ := req["spec"].(map[string]interface{})
		var aud []string
		var exp int64 = 3600
		if spec != nil {
			if a, ok := spec["audiences"].([]interface{}); ok {
				for _, v := range a {
					if s, ok := v.(string); ok {
						aud = append(aud, s)
					}
				}
			}
			switch e := spec["expirationSeconds"].(type) {
			case float64:
				exp = int64(e)
			case int64:
				exp = e
			}
		}
		// Ensure SA exists to get its UID
		sa, ok := st.getServiceAccount(ns, saName)
		if !ok {
			writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("serviceaccounts \"%s\" not found", saName)))
			return
		}
		meta, _ := sa["metadata"].(map[string]interface{})
		saUID, _ := meta["uid"].(string)
		secName := fmt.Sprintf("%s-tokenreq-%x", saName, time.Now().Unix()%0xffff)
		tok := mintServiceAccountJWT(ns, saName, saUID, secName, aud, exp)
		now := time.Now().UTC()
		resp := map[string]interface{}{
			"apiVersion": "authentication.k8s.io/v1",
			"kind":       "TokenRequest",
			"status": map[string]interface{}{
				"token":               tok,
				"expirationTimestamp": now.Add(time.Duration(exp) * time.Second).Format(time.RFC3339),
			},
		}
		writeJSON(w, 201, resp)
	})
	// Nodes: a single node object (list/watch)
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		// Provide minimal nodeInfo to satisfy version parsing in controllers
		node := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Node",
			"metadata":   map[string]interface{}{"name": "mock-node", "resourceVersion": st.nextRV()},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
				"nodeInfo": map[string]interface{}{
					"kubeletVersion":          "v1.28.1",
					"kubeProxyVersion":        "v1.28.1",
					"containerRuntimeVersion": "containerd://1.6.12",
				},
			},
		}
		if isWatch(r) {
			writeWatchEvents(w, []map[string]interface{}{node}, "v1", "Node", node["metadata"].(map[string]interface{})["resourceVersion"].(string), wantBookmark(r))
			return
		}
		writeJSON(w, 200, map[string]interface{}{"kind": "NodeList", "apiVersion": "v1", "items": []interface{}{node}})
	})
	// Nodes item: GET by name
	mux.HandleFunc("/api/v1/nodes/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/")
		if name == "" {
			writeJSON(w, 404, k8sStatus(404, "NotFound", "nodes is not a named resource"))
			return
		}
		if name != "mock-node" {
			writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("node \"%s\" not found", name)))
			return
		}
		node := map[string]interface{}{"apiVersion": "v1", "kind": "Node", "metadata": map[string]interface{}{"name": "mock-node", "resourceVersion": st.nextRV()}, "status": map[string]interface{}{"conditions": []interface{}{}}}
		writeJSON(w, 200, node)
	})
	// Namespaces collection (list/watch/create)
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		if isWatch(r) {
			list := st.listNS()
			items := make([]map[string]interface{}, 0, len(list))
			rv := ""
			for _, n := range list {
				if s, _ := n.Metadata["resourceVersion"].(string); s != "" {
					rv = s
				}
				b, _ := json.Marshal(n)
				var m map[string]interface{}
				_ = json.Unmarshal(b, &m)
				items = append(items, m)
			}
			writeWatchEvents(w, items, "v1", "Namespace", rv, wantBookmark(r))
			return
		}
		switch r.Method {
		case http.MethodGet:
			list := st.listNS()
			items := make([]interface{}, 0, len(list))
			for _, n := range list {
				items = append(items, n)
			}
			writeJSON(w, 200, map[string]interface{}{"kind": "NamespaceList", "apiVersion": "v1", "items": items})
			return
		case http.MethodPost:
			var in namespaceObj
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, 400, map[string]string{"error": "bad json"})
				return
			}
			if in.APIVersion == "" {
				in.APIVersion = "v1"
			}
			if in.Kind == "" {
				in.Kind = "Namespace"
			}
			if in.Metadata == nil {
				in.Metadata = map[string]interface{}{}
			}
			name, _ := in.Metadata["name"].(string)
			if name == "" {
				writeJSON(w, 400, map[string]string{"error": "name required"})
				return
			}
			out := st.upsertNS(&in)
			// Auto-create default service account and its token secret to match K8s behavior
			st.upsertServiceAccount(name, map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ServiceAccount",
				"metadata": map[string]interface{}{
					"name":      "default",
					"namespace": name,
				},
			})
			_ = st.ensureTokenSecretForSA(name, "default")
			writeJSON(w, 201, out)
			return
		default:
			w.WriteHeader(405)
			return
		}
	})
	// Namespaces item and namespaced resources (list/watch and basic CRUD)
	mux.HandleFunc("/api/v1/namespaces/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
		if rest == "" {
			w.WriteHeader(404)
			return
		}
		parts := strings.Split(rest, "/")
		ns := parts[0]
		if ns == "" {
			w.WriteHeader(404)
			return
		}
		// namespace object
		if len(parts) == 1 || parts[1] == "" {
			switch r.Method {
			case http.MethodGet:
				if obj, ok := st.getNS(ns); ok {
					writeJSON(w, 200, obj)
					return
				}
				writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("namespaces \"%s\" not found", ns)))
				return
			case http.MethodPut, http.MethodPatch:
				var in namespaceObj
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					writeJSON(w, 400, map[string]string{"error": "bad json"})
					return
				}
				if in.Metadata == nil {
					in.Metadata = map[string]interface{}{}
				}
				in.Metadata["name"] = ns
				out := st.upsertNS(&in)
				writeJSON(w, 200, out)
				return
			case http.MethodDelete:
				// For now, just return success for DELETE operations
				// In a real implementation, we would actually delete the namespace
				writeJSON(w, 200, map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Status",
					"status":     "Success",
					"message":    "Namespace deleted successfully",
				})
				return
			default:
				w.WriteHeader(405)
				return
			}
		}
		if len(parts) >= 2 {
			res := parts[1]
			switch res {
			case "serviceaccounts":
				// collection or named resource
				if len(parts) == 2 || parts[2] == "" {
					if isWatch(r) {
						items := []map[string]interface{}{}
						rv := ""
						for _, it := range st.listServiceAccounts(ns) {
							items = append(items, it)
							if md, _ := it["metadata"].(map[string]interface{}); md != nil {
								if v, _ := md["resourceVersion"].(string); v != "" {
									rv = v
								}
							}
						}
						writeWatchEvents(w, items, "v1", "ServiceAccount", rv, wantBookmark(r))
						return
					}
					switch r.Method {
					case http.MethodGet:
						items := []interface{}{}
						for _, it := range st.listServiceAccounts(ns) {
							items = append(items, it)
						}
						writeJSON(w, 200, map[string]interface{}{"kind": "ServiceAccountList", "apiVersion": "v1", "items": items})
						return
					case http.MethodPost:
						var in map[string]interface{}
						if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
							writeJSON(w, 400, map[string]string{"error": "bad json"})
							return
						}
						out := st.upsertServiceAccount(ns, in)
						// Auto-create token secret for impersonation SAs
						meta, _ := out["metadata"].(map[string]interface{})
						if meta != nil {
							if name, _ := meta["name"].(string); name != "" {
								if ns == "cattle-impersonation-system" || strings.HasPrefix(name, "cattle-impersonation-") {
									_ = st.ensureTokenSecretForSA(ns, name)
								}
							}
						}
						writeJSON(w, 201, out)
						return
					case http.MethodDelete:
						// Handle DELETE for service account collection (not supported in real K8s)
						writeJSON(w, 405, map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Status",
							"status":     "Failure",
							"message":    "Method not allowed",
							"reason":     "MethodNotAllowed",
							"code":       405,
						})
						return
					default:
						w.WriteHeader(405)
						return
					}
				} else {
					name := parts[2]
					switch r.Method {
					case http.MethodPost:
						// Support TokenRequest via core path: /api/v1/namespaces/{ns}/serviceaccounts/{name}/token
						if len(parts) >= 4 && parts[3] == "token" {
							var req map[string]interface{}
							_ = json.NewDecoder(r.Body).Decode(&req)
							spec, _ := req["spec"].(map[string]interface{})
							var aud []string
							var exp int64 = 3600
							if spec != nil {
								if a, ok := spec["audiences"].([]interface{}); ok {
									for _, v := range a {
										if s, ok := v.(string); ok {
											aud = append(aud, s)
										}
									}
								}
								switch e := spec["expirationSeconds"].(type) {
								case float64:
									exp = int64(e)
								case int64:
									exp = e
								}
							}
							// Ensure SA exists
							sa, ok := st.getServiceAccount(ns, name)
							if !ok {
								writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("serviceaccounts \"%s\" not found", name)))
								return
							}
							md, _ := sa["metadata"].(map[string]interface{})
							saUID, _ := md["uid"].(string)
							secName := fmt.Sprintf("%s-tokenreq-%x", name, time.Now().Unix()%0xffff)
							tok := mintServiceAccountJWT(ns, name, saUID, secName, aud, exp)
							now := time.Now().UTC()
							resp := map[string]interface{}{
								"apiVersion": "authentication.k8s.io/v1",
								"kind":       "TokenRequest",
								"status": map[string]interface{}{
									"token":               tok,
									"expirationTimestamp": now.Add(time.Duration(exp) * time.Second).Format(time.RFC3339),
								},
							}
							writeJSON(w, 201, resp)
							return
						}
					case http.MethodGet:
						if obj, ok := st.getServiceAccount(ns, name); ok {
							writeJSON(w, 200, obj)
							return
						}
						writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("serviceaccounts \"%s\" not found", name)))
						return
					case http.MethodPut, http.MethodPatch:
						// Accept JSON and common patch content types; merge minimally and upsert
						ct := strings.ToLower(r.Header.Get("Content-Type"))
						var in map[string]interface{}
						if strings.Contains(ct, "json") || ct == "" {
							dec := json.NewDecoder(r.Body)
							_ = dec.Decode(&in) // tolerate errors; we'll synthesize
						}
						if in == nil {
							in = map[string]interface{}{}
						}
						md, _ := in["metadata"].(map[string]interface{})
						if md == nil {
							md = map[string]interface{}{}
							in["metadata"] = md
						}
						md["name"] = name
						md["namespace"] = ns
						if in["apiVersion"] == nil {
							in["apiVersion"] = "v1"
						}
						if in["kind"] == nil {
							in["kind"] = "ServiceAccount"
						}
						out := st.upsertServiceAccount(ns, in)
						// Ensure token secret exists for impersonation SAs
						if name != "" && (ns == "cattle-impersonation-system" || strings.HasPrefix(name, "cattle-impersonation-")) {
							_ = st.ensureTokenSecretForSA(ns, name)
						}
						writeJSON(w, 200, out)
						return
					case http.MethodDelete:
						// Handle DELETE for individual service accounts
						if st.deleteServiceAccount(ns, name) {
							writeJSON(w, 200, map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Status",
								"status":     "Success",
								"message":    "ServiceAccount deleted successfully",
							})
						} else {
							writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("serviceaccounts \"%s\" not found", name)))
						}
						return
					default:
						w.WriteHeader(405)
						return
					}
				}
			case "secrets":
				if len(parts) == 2 || parts[2] == "" {
					if isWatch(r) {
						items := []map[string]interface{}{}
						rv := ""
						for _, it := range st.listSecrets(ns) {
							items = append(items, it)
							if md, _ := it["metadata"].(map[string]interface{}); md != nil {
								if v, _ := md["resourceVersion"].(string); v != "" {
									rv = v
								}
							}
						}
						writeWatchEvents(w, items, "v1", "Secret", rv, wantBookmark(r))
						return
					}
					switch r.Method {
					case http.MethodGet:
						items := []interface{}{}
						// Optional fieldSelector filters: support metadata.name and type
						fieldSel := r.URL.Query().Get("fieldSelector")
						var wantName, wantType string
						if fieldSel != "" {
							for _, part := range strings.Split(fieldSel, ",") {
								kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
								if len(kv) == 2 {
									k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
									if k == "metadata.name" {
										wantName = v
									}
									if k == "type" {
										wantType = v
									}
								}
							}
						}
						for _, it := range st.listSecrets(ns) {
							if wantName != "" {
								if md, _ := it["metadata"].(map[string]interface{}); md != nil {
									if n, _ := md["name"].(string); n != wantName {
										continue
									}
								} else {
									continue
								}
							}
							if wantType != "" {
								if tp, _ := it["type"].(string); tp != wantType {
									continue
								}
							}
							items = append(items, it)
						}
						writeJSON(w, 200, map[string]interface{}{"kind": "SecretList", "apiVersion": "v1", "items": items})
						return
					case http.MethodPost:
						var in map[string]interface{}
						ct := strings.ToLower(r.Header.Get("Content-Type"))
						// Best-effort decode JSON bodies; tolerate protobuf or empty bodies by synthesizing a Secret
						if strings.Contains(ct, "json") || ct == "" {
							dec := json.NewDecoder(r.Body)
							if err := dec.Decode(&in); err != nil && err != io.EOF {
								// keep 'in' nil to trigger synthesis below
								in = nil
							}
						} else {
							// Non-JSON (likely protobuf); ignore body and synthesize below
						}

						// If client is creating a service-account token secret, mint one and return it.
						// Only auto-mint on explicit SA token type (JSON) or when the body is non-JSON (protobuf path).
						var targetSA string
						explicitSAToken := false
						if in != nil {
							if t, _ := in["type"].(string); t == "kubernetes.io/service-account-token" {
								explicitSAToken = true
								if md, _ := in["metadata"].(map[string]interface{}); md != nil {
									if ann, _ := md["annotations"].(map[string]interface{}); ann != nil {
										if v, _ := ann["kubernetes.io/service-account.name"].(string); v != "" {
											targetSA = v
										}
									}
								}
							}
						}
						nonJSON := !(strings.Contains(ct, "json") || ct == "")
						if (explicitSAToken || nonJSON) && targetSA == "" {
							cands := st.listServiceAccounts(ns)
							var newest string
							var newestRV string
							for _, sa := range cands {
								md, _ := sa["metadata"].(map[string]interface{})
								name, _ := md["name"].(string)
								rv, _ := md["resourceVersion"].(string)
								if name != "" && (ns == "cattle-impersonation-system" || strings.HasPrefix(name, "cattle-impersonation-")) {
									if rv > newestRV {
										newestRV = rv
										newest = name
									}
								}
							}
							if newest != "" {
								targetSA = newest
							}
						}

						if targetSA != "" {
							// If client provided a secret name, honor it when minting
							secName := ""
							if in != nil {
								if md, _ := in["metadata"].(map[string]interface{}); md != nil {
									if v, _ := md["name"].(string); v != "" {
										secName = v
									}
								}
							}
							var created map[string]interface{}
							if secName != "" {
								created = st.populateSecretAsSAToken(ns, secName, targetSA)
							} else {
								created = st.ensureTokenSecretForSA(ns, targetSA)
							}
							if created != nil {
								writeJSON(w, 201, created)
								return
							}
						}

						// Fallback: accept and store whatever we have (or synthesize a bare Secret)
						if in == nil {
							in = map[string]interface{}{}
						}
						if in["apiVersion"] == nil {
							in["apiVersion"] = "v1"
						}
						if in["kind"] == nil {
							in["kind"] = "Secret"
						}
						out := st.upsertSecret(ns, in)
						writeJSON(w, 201, out)
						return
					case http.MethodDelete:
						// Handle DELETE for secrets collection (not supported in real K8s)
						writeJSON(w, 405, map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Status",
							"status":     "Failure",
							"message":    "Method not allowed",
							"reason":     "MethodNotAllowed",
							"code":       405,
						})
						return
					default:
						w.WriteHeader(405)
						return
					}
				} else {
					name := parts[2]
					switch r.Method {
					case http.MethodGet:
						if obj, ok := st.getSecret(ns, name); ok {
							writeJSON(w, 200, obj)
							return
						}
						writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("secrets \"%s\" not found", name)))
						return
					case http.MethodDelete:
						// Handle DELETE for individual secrets
						if st.deleteSecret(ns, name) {
							writeJSON(w, 200, map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Status",
								"status":     "Success",
								"message":    "Secret deleted successfully",
							})
						} else {
							writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("secrets \"%s\" not found", name)))
						}
						return
					default:
						w.WriteHeader(405)
						return
					}
				}
			case "resourcequotas":
				if isWatch(r) {
					writeWatchEvents(w, nil, "v1", "ResourceQuota", "1", wantBookmark(r))
					return
				}
				writeJSON(w, 200, map[string]interface{}{"kind": "ResourceQuotaList", "apiVersion": "v1", "items": []interface{}{}})
				return
			case "limitranges":
				if isWatch(r) {
					writeWatchEvents(w, nil, "v1", "LimitRange", "1", wantBookmark(r))
					return
				}
				writeJSON(w, 200, map[string]interface{}{"kind": "LimitRangeList", "apiVersion": "v1", "items": []interface{}{}})
				return
			case "roles", "rolebindings":
				// handle named resource GET/PUT/PATCH
				if len(parts) >= 3 && parts[2] != "" {
					name := parts[2]
					namedRes := parts[1]
					switch namedRes {
					case "roles":
						switch r.Method {
						case http.MethodGet:
							if obj, ok := st.getRole(ns, name); ok {
								writeJSON(w, 200, obj)
								return
							}
							writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("roles \"%s\" not found", name)))
							return
						case http.MethodPut, http.MethodPatch:
							var in map[string]interface{}
							_ = json.NewDecoder(r.Body).Decode(&in)
							if in == nil {
								in = map[string]interface{}{}
							}
							md, _ := in["metadata"].(map[string]interface{})
							if md == nil {
								md = map[string]interface{}{}
								in["metadata"] = md
							}
							md["name"] = name
							md["namespace"] = ns
							out := st.upsertRole(ns, in)
							writeJSON(w, 200, out)
							return
						case http.MethodDelete:
							// Handle DELETE for individual roles
							if st.deleteRole(ns, name) {
								writeJSON(w, 200, map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Status",
									"status":     "Success",
									"message":    "Role deleted successfully",
								})
							} else {
								writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("roles \"%s\" not found", name)))
							}
							return
						default:
							w.WriteHeader(405)
							return
						}
					case "rolebindings":
						switch r.Method {
						case http.MethodGet:
							if obj, ok := st.getRoleBinding(ns, name); ok {
								writeJSON(w, 200, obj)
								return
							}
							writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("rolebindings \"%s\" not found", name)))
							return
						case http.MethodPut, http.MethodPatch:
							var in map[string]interface{}
							_ = json.NewDecoder(r.Body).Decode(&in)
							if in == nil {
								in = map[string]interface{}{}
							}
							md, _ := in["metadata"].(map[string]interface{})
							if md == nil {
								md = map[string]interface{}{}
								in["metadata"] = md
							}
							md["name"] = name
							md["namespace"] = ns
							out := st.upsertRoleBinding(ns, in)
							writeJSON(w, 200, out)
							return
						case http.MethodDelete:
							// Handle DELETE for individual rolebindings
							if st.deleteRoleBinding(ns, name) {
								writeJSON(w, 200, map[string]interface{}{
									"apiVersion": "v1",
									"kind":       "Status",
									"status":     "Success",
									"message":    "RoleBinding deleted successfully",
								})
							} else {
								writeJSON(w, 404, k8sStatus(404, "NotFound", fmt.Sprintf("rolebindings \"%s\" not found", name)))
							}
							return
						default:
							w.WriteHeader(405)
							return
						}
					}
				}
				// otherwise handled above
			default:
				http.NotFound(w, r)
				return
			}
		}
	})

	// Minimal OpenAPI v2 stub to prevent validation failures when clients try to fetch it.
	mux.HandleFunc("/openapi/v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		// Provide a tiny, valid JSON object; clients primarily need 200 to proceed in this simulation.
		_, _ = w.Write([]byte(`{"swagger":"2.0","info":{"title":"simulated","version":"v0"}}`))
	})

	// Add OpenAPI v3 endpoint to fix 404 errors
	mux.HandleFunc("/openapi/v3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		// Provide OpenAPI v3 spec to satisfy kubectl and other clients
		_, _ = w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"simulated","version":"v0"},"paths":{}}`))
	})

	// Start HTTPS server
	addr := ":" + strconv.Itoa(port)
	cert, caPEM, err := selfSignedCert("localhost")
	if err != nil {
		log.Fatalf("cert: %v", err)
	}
	serverCACertPEM = caPEM

	// Optional request access logging (enabled when SIM_APISERVER_DEBUG is set)
	handler := http.Handler(mux)
	if os.Getenv("SIM_APISERVER_DEBUG") != "" {
		handler = withAccessLog(mux, logAccessf)
		log.Printf("SIM_APISERVER_DEBUG enabled: access logs will be written to stderr%s",
			func() string {
				if accessLogFile != nil {
					return " and ~/.kwok/clusters/" + clusterName + "/sim-access.log"
				}
				return ""
			}())
	}

	srv := &http.Server{Addr: addr, Handler: handler, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
	log.Printf("simulated-apiserver listening on https://127.0.0.1:%d", port)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// withAccessLog wraps a handler and logs method, path, status, and duration
func withAccessLog(next http.Handler, sink func(string, ...interface{})) http.Handler {
	if sink == nil {
		sink = func(format string, args ...interface{}) { log.Printf(format, args...) }
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, code: 200}
		// Log request start with minimal headers for correlation
		ua := r.Header.Get("User-Agent")
		auth := r.Header.Get("Authorization")
		authPrefix := ""
		if auth != "" {
			if len(auth) > 16 {
				authPrefix = auth[:16] + "..."
			} else {
				authPrefix = auth
			}
		}
		sink("REQ %s %s from %s UA=%q Auth=%q", r.Method, r.URL.String(), r.RemoteAddr, ua, authPrefix)
		defer func() {
			dur := time.Since(start)
			sink("RES %s %s -> %d in %s", r.Method, r.URL.String(), lrw.code, dur)
		}()
		next.ServeHTTP(lrw, r)
	})
}
