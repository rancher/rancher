package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHelmClient_RespectsProxyEnv(t *testing.T) {
	proxyCalled := make(chan struct{}, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	os.Setenv("HTTP_PROXY", proxy.URL)
	os.Setenv("HTTPS_PROXY", proxy.URL)
	defer os.Unsetenv("HTTP_PROXY")
	defer os.Unsetenv("HTTPS_PROXY")

	client, err := HelmClient(nil, nil, false, false, "https://example.com")
	if err != nil {
		t.Fatalf("HelmClient failed: %v", err)
	}

	// Perform a dummy HTTP request using the OCI client's configured transport
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	_, _ = client.Do(req)

	// Check if proxy was actually used
	select {
	case <-proxyCalled:
		t.Log("HelmClient respected proxy environment")
	case <-time.After(1 * time.Second):
		t.Error("HelmClient did not use proxy")
	}
}

func TestAttachBasicAuthHeader(t *testing.T) {
	tests := []struct {
		testName               string
		disableSameOriginCheck bool
		resourcePath           string
		repositoryURL          string
		requestURL             string
		redirectStatusCode     int
		redirectURL            string
		expectedPass           bool
	}{
		{
			"Download index.yaml from repository with disableSameOriginCheck=false",
			false,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download index.yaml from repository with disableSameOriginCheck=true",
			true,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download index.yaml from repository with redirect to sub domain with disableSameOriginCheck=false",
			false,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://storage.charts.rancher.io/repository",
			true,
		},
		{
			"Download index.yaml from repository with redirect to sub domain with disableSameOriginCheck=true",
			true,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			308,
			"https://storage.charts.rancher.io/repository",
			true,
		},
		{
			"Download index.yaml from different origin url redirect with redirect with disableSameOriginCheck=false",
			false,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.io/repository",
			false,
		},
		{
			"Download index.yaml from different origin url redirect with redirect with disableSameOriginCheck=true",
			true,
			"/index.yaml",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			308,
			"https://blobstorage.io/repository",
			true,
		},
		{
			"Download chart from repository with disableSameOriginCheck=false",
			false,
			"/assets/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download chart from repository with disableSameOriginCheck=true",
			true,
			"/assets/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download chart from repository with redirect to sub domain with disableSameOriginCheck=false",
			false,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.charts.rancher.io/repository",
			true,
		},
		{
			"Download chart from repository with redirect to sub domain with disableSameOriginCheck=true",
			true,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.charts.rancher.io/repository",
			true,
		},
		{
			"Download chart from different origin url redirect with disableSameOriginCheck=false",
			false,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.io/repository",
			false,
		},
		{
			"Download chart from different origin url redirect with disableSameOriginCheck=true",
			true,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.io/repository",
			true,
		},
		{
			"Download chart from different origin url with disableSameOriginCheck=false",
			false,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://localhost.charts.io/repository",
			200,
			"",
			false,
		},
		{
			"Download chart from different origin url with disableSameOriginCheck=true",
			true,
			"/_blob/nginx-sample-1.1.0.tgz",
			"https://charts.rancher.io",
			"https://localhost.charts.io/repository",
			200,
			"",
			true,
		},
		{
			"Download icon from repository with disableSameOriginCheck=false",
			false,
			"/assets/logos/fleet.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download icon from repository with disableSameOriginCheck=true",
			true,
			"/assets/logos/fleet.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			200,
			"",
			true,
		},
		{
			"Download icon from repository with redirect to sub domain with disableSameOriginCheck=false",
			false,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.charts.rancher.io/repository",
			true,
		},
		{
			"Download icon from repository with redirect to sub domain with disableSameOriginCheck=true",
			true,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"https://blobstorage.charts.rancher.io/repository",
			true,
		},
		{
			"Download icon from different origin url redirect with disableSameOriginCheck=false",
			false,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"http://blobstorage.io/repository",
			false,
		},
		{
			"Download icon from different origin url redirect with disableSameOriginCheck=true",
			true,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			307,
			"http://blobstorage.io/repository",
			true,
		},
		{
			"Download icon from different origin url with disableSameOriginCheck=false",
			false,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://cattle.charts.io",
			200,
			"",
			false,
		},
		{
			"Download icon from different origin url with disableSameOriginCheck=true",
			true,
			"/assets/logos/istio.svg",
			"https://charts.rancher.io",
			"https://charts.cattle.io",
			200,
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			parsedRequestURL, err := url.Parse(tt.requestURL + tt.resourcePath)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			repositoryRequest := &http.Request{
				URL:      parsedRequestURL,
				Response: nil,
			}
			if tt.redirectURL != "" {
				resp := &http.Response{
					StatusCode: tt.redirectStatusCode,
				}
				redirectRequestURL, err := url.Parse(tt.redirectURL + tt.resourcePath)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					t.FailNow()
				}
				repositoryRequest.URL = redirectRequestURL
				repositoryRequest.Response = resp
			}
			attachHeader, _ := shouldAttachBasicAuthHeader(tt.repositoryURL, tt.disableSameOriginCheck, repositoryRequest)
			assert.Equal(t, tt.expectedPass, attachHeader)
			t.Logf("Expected %v, got %v", tt.expectedPass, attachHeader)
		})
	}
}

func TestIsDomainOrSubdomain(t *testing.T) {
	tests := []struct {
		testName     string
		repoURL      string
		requestURL   string
		expectedPass bool
	}{
		{
			"exactly matching urls",
			"https://charts.rancher.io",
			"https://charts.rancher.io",
			true,
		},
		{
			"exactly matching urls with matching paths",
			"https://charts.rancher.io/path/here",
			"https://charts.rancher.io/path/here",
			true,
		},
		{
			"matching domains, but mismatch schema",
			"https://charts.rancher.io",
			"http://charts.rancher.io",
			false,
		},
		{
			"matching domains",
			"https://123.123.12.1:8443",
			"https://123.123.12.1:8443/path/here",
			true,
		},
		{
			"sub domain does not match, but domains match",
			"https://rancher.io",
			"https://assets.rancher.io",
			true,
		},
		{
			"mismatch domains",
			"https://charts.rancher.io",
			"https://other.rancher.io",
			false,
		},
		{
			"no matching urls",
			"https://rancher.com",
			"https://assets.rancher.io",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			repoURL, err := url.Parse(tt.repoURL)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			reqURL, err := url.Parse(tt.requestURL)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			isSubDomainOrDomain := isDomainOrSubdomain(reqURL, repoURL)
			assert.Equal(t, tt.expectedPass, isSubDomainOrDomain)
			t.Logf("Expected %v, got %v", tt.expectedPass, isSubDomainOrDomain)
		})
	}
}
