package main

import (
	"crypto/rand"
	crsa "crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
}

func newState() *stateStore {
	s := &stateStore{namespaces: map[string]*namespaceObj{}, rv: 1}
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
	s.mu.RLock(); defer s.mu.RUnlock()
	out := make([]*namespaceObj, 0, len(s.namespaces))
	for _, v := range s.namespaces { out = append(out, v) }
	return out
}

func (s *stateStore) getNS(name string) (*namespaceObj, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	v, ok := s.namespaces[name]
	return v, ok
}

// upsertNS applies a full-object update. Accept updates even when resourceVersion is stale by merging and bumping rv.
func (s *stateStore) upsertNS(in *namespaceObj) *namespaceObj {
	if in == nil { return nil }
	name, _ := in.Metadata["name"].(string)
	if name == "" { return nil }
	s.mu.Lock(); defer s.mu.Unlock()
	cur, ok := s.namespaces[name]
	newRV := s.nextRV()
	if !ok {
		// create
		if in.Metadata == nil { in.Metadata = map[string]interface{}{} }
		in.Metadata["resourceVersion"] = newRV
		if in.Status == nil { in.Status = map[string]interface{}{"phase": "Active"} }
		s.namespaces[name] = in
		return in
	}
	// merge metadata (labels etc), override spec fields from input
	if in.Metadata == nil { in.Metadata = map[string]interface{}{} }
	// Merge labels
	curLabels, _ := cur.Metadata["labels"].(map[string]interface{})
	inLabels, _ := in.Metadata["labels"].(map[string]interface{})
	if curLabels == nil { curLabels = map[string]interface{}{} }
	for k, v := range inLabels { curLabels[k] = v }
	// Build updated object
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

func selfSignedCert(host string) (tls.Certificate, error) {
	priv, err := crsa.GenerateKey(rand.Reader, 2048)
	if err != nil { return tls.Certificate{}, err }
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{CommonName: host, Organization: []string{"simulated-apiserver"}},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:  []string{"localhost", host},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		IsCA: true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil { return tls.Certificate{}, err }
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return tls.X509KeyPair(certPEM, keyPEM)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	var port int
	var dbPath string
	flag.IntVar(&port, "port", 0, "listen port")
	flag.StringVar(&dbPath, "db", "", "database path (optional)")
	flag.Parse()
	if port == 0 { log.Fatal("--port is required") }
	if dbPath != "" {
		// Ensure directory and create file if missing (we don’t actually use it yet)
		_ = os.MkdirAll(path.Dir(dbPath), 0755)
		f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_RDWR, 0644)
		if err == nil { _ = f.Close() }
	}

	st := newState()

	mux := http.NewServeMux()
	// Health
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })

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
			"kind": "APIGroupList", "apiVersion": "v1",
			"groups": []map[string]interface{}{
				{"name": "rbac.authorization.k8s.io", "versions": []map[string]interface{}{{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}}, "preferredVersion": map[string]interface{}{"groupVersion": "rbac.authorization.k8s.io/v1", "version": "v1"}},
			},
		})
	})
	// Discovery: generic /apis/{group}/{version} to avoid 404s during probing
	mux.HandleFunc("/apis/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/apis/")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			writeJSON(w, 200, map[string]interface{}{
				"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": parts[0] + "/" + parts[1],
				"resources": []interface{}{},
			})
			return
		}
		http.NotFound(w, r)
	})
	// Discovery: /apis/rbac.authorization.k8s.io/v1
	mux.HandleFunc("/apis/rbac.authorization.k8s.io/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "rbac.authorization.k8s.io/v1",
			"resources": []map[string]interface{}{
				{"name": "clusterroles", "namespaced": false, "kind": "ClusterRole"},
				{"name": "clusterrolebindings", "namespaced": false, "kind": "ClusterRoleBinding"},
				{"name": "roles", "namespaced": true, "kind": "Role"},
				{"name": "rolebindings", "namespaced": true, "kind": "RoleBinding"},
			},
		})
	})
	// Discovery: /api/v1
	mux.HandleFunc("/api/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "v1",
			"resources": []map[string]interface{}{
				{"name": "namespaces", "namespaced": false, "kind": "Namespace"},
				{"name": "nodes", "namespaced": false, "kind": "Node"},
				{"name": "serviceaccounts", "namespaced": true, "kind": "ServiceAccount"},
				{"name": "secrets", "namespaced": true, "kind": "Secret"},
				{"name": "resourcequotas", "namespaced": true, "kind": "ResourceQuota"},
				{"name": "limitranges", "namespaced": true, "kind": "LimitRange"},
			},
		})
	})
	// Nodes: a single node object
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		node := map[string]interface{}{"apiVersion": "v1", "kind": "Node", "metadata": map[string]interface{}{"name": "mock-node"}, "status": map[string]interface{}{"conditions": []interface{}{}}}
		writeJSON(w, 200, map[string]interface{}{"kind": "NodeList", "apiVersion": "v1", "items": []interface{}{node}})
	})

	// Namespaces collection
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list := st.listNS()
			items := make([]interface{}, 0, len(list))
			for _, n := range list { items = append(items, n) }
			writeJSON(w, 200, map[string]interface{}{"kind": "NamespaceList", "apiVersion": "v1", "items": items})
			return
		case http.MethodPost:
			var in namespaceObj
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeJSON(w, 400, map[string]string{"error": "bad json"}); return }
			if in.APIVersion == "" { in.APIVersion = "v1" }
			if in.Kind == "" { in.Kind = "Namespace" }
			if in.Metadata == nil { in.Metadata = map[string]interface{}{} }
			name, _ := in.Metadata["name"].(string)
			if name == "" { writeJSON(w, 400, map[string]string{"error": "name required"}); return }
			out := st.upsertNS(&in)
			writeJSON(w, 201, out)
			return
		default:
			w.WriteHeader(405)
			return
		}
	})

	// Namespaces item and minimal namespaced resource lists
	mux.HandleFunc("/api/v1/namespaces/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
		if rest == "" { w.WriteHeader(404); return }
		parts := strings.Split(rest, "/")
		ns := parts[0]
		if ns == "" { w.WriteHeader(404); return }
		// if only namespace path, handle object GET/PUT/PATCH
		if len(parts) == 1 || parts[1] == "" {
			switch r.Method {
			case http.MethodGet:
				if obj, ok := st.getNS(ns); ok { writeJSON(w, 200, obj); return }
				writeJSON(w, 404, map[string]string{"error": "not found"})
				return
			case http.MethodPut, http.MethodPatch:
				var in namespaceObj
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil { writeJSON(w, 400, map[string]string{"error": "bad json"}); return }
				if in.Metadata == nil { in.Metadata = map[string]interface{}{} }
				in.Metadata["name"] = ns
				out := st.upsertNS(&in)
				writeJSON(w, 200, out)
				return
			default:
				w.WriteHeader(405)
				return
			}
		}
		// namespaced resources minimal support
		if len(parts) >= 2 {
			res := parts[1]
			switch res {
			case "serviceaccounts":
				writeJSON(w, 200, map[string]interface{}{"kind": "ServiceAccountList", "apiVersion": "v1", "items": []interface{}{}})
				return
			case "secrets":
				writeJSON(w, 200, map[string]interface{}{"kind": "SecretList", "apiVersion": "v1", "items": []interface{}{}})
				return
			default:
				http.NotFound(w, r)
				return
			}
		}
	})

	// Start HTTPS server
	addr := ":" + strconv.Itoa(port)
	cert, err := selfSignedCert("localhost")
	if err != nil { log.Fatalf("cert: %v", err) }
	srv := &http.Server{Addr: addr, Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
	log.Printf("simulated-apiserver listening on https://127.0.0.1:%d", port)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
