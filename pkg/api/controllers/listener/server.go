package listener

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

const (
	httpsMode = "https"
	httpMode  = "http"
	acmeMode  = "acme"
)

type Server struct {
	sync.Mutex

	listenConfigs       v3.ListenConfigInterface
	handler             HandlerGetter
	httpPort, httpsPort int
	certs               map[string]*tls.Certificate
	ips                 *lru.Cache

	listeners    []net.Listener
	servers      []*http.Server
	activeConfig *v3.ListenConfig
	activeMode   string

	// dynamic config change on refresh
	activeCert  *tls.Certificate
	activeCA    *x509.Certificate
	activeCAKey *rsa.PrivateKey
	domains     map[string]bool
	tos         []string
	tosAll      bool
}

func NewServer(ctx context.Context, listenConfigs v3.ListenConfigInterface, handler HandlerGetter, httpPort, httpsPort int) *Server {
	s := &Server{
		listenConfigs: listenConfigs,
		handler:       handler,
		httpPort:      httpPort,
		httpsPort:     httpsPort,
		certs:         map[string]*tls.Certificate{},
	}

	s.ips, _ = lru.New(20)

	go s.start(ctx)
	return s
}

func (s *Server) updateIPs(savedIPs map[string]bool) map[string]bool {
	if s.activeCert != nil || s.activeConfig == nil {
		return savedIPs
	}

	cfg, err := s.listenConfigs.Get(s.activeConfig.Name, v1.GetOptions{})
	if err != nil {
		return savedIPs
	}

	certs := map[string]string{}
	for key, cert := range s.certs {
		certs[key] = certToString(cert)
	}

	if !reflect.DeepEqual(certs, cfg.GeneratedCerts) {
		cfg = cfg.DeepCopy()
		cfg.GeneratedCerts = certs
		cfg, err = s.listenConfigs.Update(cfg)
		if err != nil {
			return savedIPs
		}
	}

	allIPs := map[string]bool{}
	for _, k := range s.ips.Keys() {
		s, ok := k.(string)
		if ok {
			allIPs[s] = true
		}
	}

	a, b, _ := set.Diff(allIPs, savedIPs)
	if len(a) == 0 && len(b) == 0 {
		return savedIPs
	}

	cfg.KnownIPs = nil
	for k := range allIPs {
		cfg = cfg.DeepCopy()
		cfg.KnownIPs = append(cfg.KnownIPs, k)
	}

	_, err = s.listenConfigs.Update(cfg)
	if err != nil {
		return savedIPs
	}

	return allIPs
}

func (s *Server) start(ctx context.Context) {
	savedIPs := map[string]bool{}
	for {
		savedIPs = s.updateIPs(savedIPs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (s *Server) Disable(config *v3.ListenConfig) {
	if s.activeConfig == nil {
		return
	}

	if s.activeConfig.UID == config.UID {
		s.activeConfig = nil
	}
}

func (s *Server) Enable(config *v3.ListenConfig) (bool, error) {
	s.Lock()
	defer s.Unlock()

	if s.activeConfig != nil && s.activeConfig.CreationTimestamp.Before(&config.CreationTimestamp) {
		return false, nil
	}

	s.domains = map[string]bool{}
	for _, d := range config.Domains {
		s.domains[d] = true
	}

	s.tos = config.TOS
	s.tosAll = len(config.TOS) == 0 || slice.ContainsString(config.TOS, "auto")

	if config.Key != "" && config.Cert != "" {
		cert, err := tls.X509KeyPair([]byte(config.Cert), []byte(config.Key))
		if err != nil {
			return false, err
		}
		s.activeCert = &cert
	}

	if config.CACert != "" && config.CAKey != "" {
		cert, err := tls.X509KeyPair([]byte(config.CACert), []byte(config.CAKey))
		if err != nil {
			return false, err
		}
		s.activeCAKey = cert.PrivateKey.(*rsa.PrivateKey)

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return false, err
		}
		s.activeCA = x509Cert
	}

	if s.activeConfig == nil || config.Mode != s.activeMode {
		return true, s.reload(config)
	}

	return true, nil
}

func (s *Server) hostPolicy(ctx context.Context, host string) error {
	s.Lock()
	defer s.Unlock()

	if s.domains[host] {
		return nil
	}

	return errors.New("acme/autocert: host not configured")
}

func (s *Server) prompt(tos string) bool {
	s.Lock()
	defer s.Unlock()

	if s.tosAll {
		return true
	}

	return slice.ContainsString(s.tos, tos)
}

func (s *Server) Shutdown() error {
	for _, listener := range s.listeners {
		if err := listener.Close(); err != nil {
			return err
		}
	}
	s.listeners = nil

	for _, server := range s.servers {
		go server.Shutdown(context.Background())
	}
	s.servers = nil

	return nil
}

func (s *Server) reload(config *v3.ListenConfig) error {
	if err := s.Shutdown(); err != nil {
		return err
	}

	switch config.Mode {
	case acmeMode:
		if err := s.serveACME(config); err != nil {
			return err
		}
	case httpMode:
		if err := s.serveHTTP(config); err != nil {
			return err
		}
	case httpsMode:
		if err := s.serveHTTPS(config); err != nil {
			return err
		}
	}

	for _, ipStr := range config.KnownIPs {
		ip := net.ParseIP(ipStr)
		if len(ip) > 0 {
			s.ips.ContainsOrAdd(ipStr, ip)
		}
	}

	for key, certString := range config.GeneratedCerts {
		cert := stringToCert(certString)
		if cert != nil {
			s.certs[key] = cert
		}
	}

	s.activeMode = config.Mode
	s.activeConfig = config
	return nil
}

func (s *Server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.Lock()
	defer s.Unlock()

	if s.activeCert != nil {
		return s.activeCert, nil
	}

	mapKey := hello.ServerName
	cn := hello.ServerName
	dnsNames := []string{cn}
	ipBased := false
	var ips []net.IP

	if cn == "" {
		mapKey = fmt.Sprintf("local/%d", s.ips.Len())
		ipBased = true
	}

	serverNameCert, ok := s.certs[mapKey]
	if ok {
		return serverNameCert, nil
	}

	if ipBased {
		cn = "cattle"
		for _, ipStr := range s.ips.Keys() {
			ip := net.ParseIP(ipStr.(string))
			if len(ip) > 0 {
				ips = append(ips, ip)
			}
		}
	}

	cfg := cert.Config{
		CommonName:   cn,
		Organization: s.activeCA.Subject.Organization,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: dnsNames,
			IPs:      ips,
		},
	}

	key, err := cert.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := cert.NewSignedCert(cfg, key, s.activeCA, s.activeCAKey)
	if err != nil {
		return nil, err
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{
			cert.Raw,
		},
		PrivateKey: key,
	}

	s.certs[mapKey] = tlsCert
	return tlsCert, nil
}

func (s *Server) cacheIPHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		h, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			h = req.Host
		}

		ip := net.ParseIP(h)
		if len(ip) > 0 {
			s.ips.ContainsOrAdd(h, ip)
		}

		handler.ServeHTTP(resp, req)
	})
}

func (s *Server) serveHTTPS(config *v3.ListenConfig) error {
	conf := &tls.Config{
		GetCertificate: s.getCertificate,
	}
	listener, err := s.newListener(s.httpsPort, conf)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler: s.Handler(),
	}

	if s.activeConfig == nil {
		server.Handler = s.cacheIPHandler(server.Handler)
	}

	s.servers = append(s.servers, server)
	s.startServer(listener, server)

	httpListener, err := s.newListener(s.httpPort, nil)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler: httpRedirect(s.Handler()),
	}

	if s.activeConfig == nil {
		httpServer.Handler = s.cacheIPHandler(httpServer.Handler)
	}

	s.servers = append(s.servers, httpServer)
	s.startServer(httpListener, httpServer)

	return nil
}

// Approach taken from letsencrypt, except manglePort is specific to us
func httpRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-Forwarded-Proto") == "https" {
				next.ServeHTTP(rw, r)
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				http.Error(rw, "Use HTTPS", http.StatusBadRequest)
				return
			}
			target := "https://" + manglePort(r.Host) + r.URL.RequestURI()
			http.Redirect(rw, r, target, http.StatusFound)
		})
}

func manglePort(hostport string) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return hostport
	}

	portInt = ((portInt / 1000) * 1000) + 443

	return net.JoinHostPort(host, strconv.Itoa(portInt))
}

func (s *Server) serveHTTP(config *v3.ListenConfig) error {
	listener, err := s.newListener(s.httpPort, nil)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler: s.Handler(),
	}
	s.servers = append(s.servers, server)
	s.startServer(listener, server)
	return nil
}

func (s *Server) startServer(listener net.Listener, server *http.Server) {
	go func() {
		if err := server.Serve(listener); err != nil {
			logrus.Errorf("server on %v returned err: %v", listener.Addr(), err)
		}
	}()
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		h := s.handler()
		if h == nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
		} else {
			h.ServeHTTP(rw, req)
		}
	})
}

func (s *Server) newListener(port int, config *tls.Config) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	l = tcpKeepAliveListener{l.(*net.TCPListener)}

	if config != nil {
		l = tls.NewListener(l, config)
	}

	s.listeners = append(s.listeners, l)
	logrus.Info("Listening on ", addr)
	return l, nil
}

func (s *Server) serveACME(config *v3.ListenConfig) error {
	manager := autocert.Manager{
		Cache:      autocert.DirCache("certs-cache"),
		Prompt:     s.prompt,
		HostPolicy: s.hostPolicy,
	}
	conf := &tls.Config{
		GetCertificate: manager.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}

	httpsListener, err := s.newListener(s.httpsPort, conf)
	if err != nil {
		return err
	}

	httpListener, err := s.newListener(s.httpPort, nil)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler: manager.HTTPHandler(nil),
	}
	s.servers = append(s.servers, httpServer)
	go func() {
		if err := httpServer.Serve(httpListener); err != nil {
			logrus.Errorf("http server returned err: %v", err)
		}
	}()

	httpsServer := &http.Server{
		Handler: s.Handler(),
	}
	s.servers = append(s.servers, httpsServer)
	go func() {
		if err := httpsServer.Serve(httpsListener); err != nil {
			logrus.Errorf("https server returned err: %v", err)
		}
	}()

	return nil
}

func stringToCert(certString string) *tls.Certificate {
	parts := strings.Split(certString, "#")
	if len(parts) != 2 {
		return nil
	}

	cert, key := parts[0], parts[1]
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(keyBytes)
	if err != nil {
		return nil
	}

	certBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return nil
	}

	return &tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  rsaKey,
	}
}

func certToString(cert *tls.Certificate) string {
	certString := base64.StdEncoding.EncodeToString(cert.Certificate[0])
	keyString := base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey)))
	return certString + "#" + keyString
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
