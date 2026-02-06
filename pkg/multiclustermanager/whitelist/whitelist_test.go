package whitelist

import (
	"strings"
	"sync"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testProxyEndpointOne = mgmtv3.ProxyEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "54321",
			Name: "testProxyEndpointOne",
		},
		Spec: mgmtv3.ProxyEndpointSpec{
			Routes: []mgmtv3.ProxyEndpointRoute{
				{
					Domain: "example.com",
				},
			},
		},
	}
	testProxyEndpointTwo = mgmtv3.ProxyEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "12345",
			Name: "testProxyEndpointTwo",
		},
		Spec: mgmtv3.ProxyEndpointSpec{
			Routes: []mgmtv3.ProxyEndpointRoute{
				{
					Domain: "other.com",
				},
				{
					Domain: "example.com",
				},
			},
		},
	}
)

func Test_ProxyWhiteList(t *testing.T) {
	test := []struct {
		name                   string
		endpointsToAdd         []*mgmtv3.ProxyEndpoint
		endpointsToRemove      []*mgmtv3.ProxyEndpoint
		whiteListEnvVarDomains []string
		expectedDomains        []string
	}{
		{
			name:                   "No custom endpointsToAdd",
			endpointsToAdd:         []*mgmtv3.ProxyEndpoint{},
			endpointsToRemove:      []*mgmtv3.ProxyEndpoint{},
			whiteListEnvVarDomains: []string{"aws.com", "myapi.com", "myenvvar.com"},
			expectedDomains:        []string{"aws.com", "myapi.com", "myenvvar.com"},
		},
		{
			name: "Custom Endpoint With Whitelisted Domains",
			endpointsToAdd: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
			},
			whiteListEnvVarDomains: []string{"aws.com", "myapi.com", "myenvvar.com"},
			expectedDomains:        []string{"aws.com", "myapi.com", "myenvvar.com", "example.com"},
		},
		{
			name: "Two Custom Endpoints With duplicate domains",
			endpointsToAdd: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
				&testProxyEndpointTwo,
			},
			expectedDomains: []string{"example.com", "other.com"},
		},
		{
			name: "Remove one Custom Endpoint With duplicate domains",
			endpointsToAdd: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
				&testProxyEndpointTwo,
			},
			endpointsToRemove: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
			},
			expectedDomains: []string{"example.com", "other.com"},
		},
		{
			name: "Remove both Custom Endpoints",
			endpointsToAdd: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
				&testProxyEndpointTwo,
			},
			endpointsToRemove: []*mgmtv3.ProxyEndpoint{
				&testProxyEndpointOne,
				&testProxyEndpointTwo,
			},
			expectedDomains: []string{},
		},
	}

	// Don't actually use settings.WhitelistDomain in case some other tests rely on it.
	testWhiteListDomainSetting := settings.NewSetting("whitelisteddomains", "")
	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {

			// clear out old proxy and env settings
			assert.NoErrorf(t, testWhiteListDomainSetting.Set(""), "failed to set WhitelistDomain env var")
			if len(tc.whiteListEnvVarDomains) > 0 {
				assert.NoErrorf(t, testWhiteListDomainSetting.Set(strings.Join(tc.whiteListEnvVarDomains, ",")), "failed to set WhitelistDomain env var")
			}

			Proxy = ProxyAcceptList{
				accept:           map[string]map[string]struct{}{},
				RWMutex:          sync.RWMutex{},
				envSettingGetter: testWhiteListDomainSetting.Get,
			}

			for _, ep := range tc.endpointsToAdd {
				_, err := Proxy.onChangeEndpoint("", ep)
				if err != nil {
					t.Fatalf("Error adding proxy endpoint: %v", err)
				}
			}

			// Remove endpoints
			for _, ep := range tc.endpointsToRemove {
				_, err := Proxy.onRemoveEndpoint("", ep)
				if err != nil {
					t.Fatalf("Error removing proxy endpoint: %v", err)
				}
			}

			// Get current whitelist
			domains := Proxy.Get()

			// Check expected vs actual
			domainMap := make(map[string]bool)
			for _, d := range domains {
				domainMap[d] = true
			}

			for _, expectedDomain := range tc.expectedDomains {
				if !domainMap[expectedDomain] {
					t.Errorf("Expected domain %s not found in whitelist", expectedDomain)
				}
			}

			if len(domains) != len(tc.expectedDomains) {
				t.Errorf("Expected %d domains, but got %d: %v\n%v", len(tc.expectedDomains), len(domains), domains, tc.expectedDomains)
			}
		})
	}
}
