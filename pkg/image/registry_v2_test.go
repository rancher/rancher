package image

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const fakeToken = "fake-token"

// fakeRegistry is a registry that answers the HEAD of the manifests of the images it was given, the way the ones of
// SUSE do: with a token that anyone can ask for, when it requires one.
type fakeRegistry struct {
	server       *httptest.Server
	requireToken bool

	mu       sync.Mutex
	statuses map[string][]int // the statuses to answer for "repository:tag", in order, and the last one for the rest
	requests map[string]int
	tokens   int
}

func newFakeRegistry(t *testing.T, requireToken bool) *fakeRegistry {
	t.Helper()
	f := &fakeRegistry{requireToken: requireToken, statuses: map[string][]int{}, requests: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

// host is the address of the registry, without a scheme, as it is in an image.
func (f *fakeRegistry) host() string {
	return strings.TrimPrefix(f.server.URL, "http://")
}

// has makes the registry answer the HEAD of the image with the statuses, 200 when there are none.
func (f *fakeRegistry) has(image string, statuses ...int) {
	if len(statuses) == 0 {
		statuses = []int{http.StatusOK}
	}
	f.statuses[image] = statuses
}

func (f *fakeRegistry) requestsFor(image string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[image]
}

func (f *fakeRegistry) challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="fake"`, f.server.URL))
	w.WriteHeader(http.StatusUnauthorized)
}

func (f *fakeRegistry) authorized(r *http.Request) bool {
	return !f.requireToken || r.Header.Get("Authorization") == "Bearer "+fakeToken
}

func (f *fakeRegistry) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/token":
		f.mu.Lock()
		f.tokens++
		f.mu.Unlock()
		fmt.Fprintf(w, `{"token":%q,"expires_in":300}`, fakeToken)
	case r.URL.Path == "/v2/":
		if !f.authorized(r) {
			f.challenge(w)
			return
		}
		w.WriteHeader(http.StatusOK)
	case strings.Contains(r.URL.Path, "/manifests/"):
		if !f.authorized(r) {
			f.challenge(w)
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/v2/"), "/manifests/", 2)
		image := parts[0] + ":" + parts[1]
		f.mu.Lock()
		statuses, known := f.statuses[image]
		status := http.StatusNotFound
		if known {
			status = statuses[0]
			if len(statuses) > 1 {
				f.statuses[image] = statuses[1:]
			}
		}
		f.requests[image]++
		f.mu.Unlock()
		if status == http.StatusOK {
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("a", 64))
			w.Header().Set("Content-Length", "100")
		}
		w.WriteHeader(status)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// useFakeRegistries makes the requests to a registry go without TLS, and wait a millisecond between the attempts.
func useFakeRegistries(t *testing.T) {
	t.Helper()
	plainHTTP, policy := registryPlainHTTP, registryRetryPolicy
	registryPlainHTTP = true
	registryRetryPolicy = func() retry.Policy {
		return &retry.GenericPolicy{
			Retryable: retryable,
			Backoff:   retry.ExponentialBackoff(time.Millisecond, 1, 0),
			MinWait:   time.Millisecond,
			MaxWait:   time.Millisecond,
			MaxRetry:  registryAttempts - 1,
		}
	}
	t.Cleanup(func() { registryPlainHTTP, registryRetryPolicy = plainHTTP, policy })
}

func fleetImage() ChartImage {
	return ChartImage{Image: "rancher/fleet:v0.17.0", Sources: []string{"fleet:111.0.1"}}
}

func TestCheckImagesExistAcceptsTheImagesThatTheRegistryHas(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)
	registry.has("rancher/fleet:v0.17.0")
	registry.has("rancher/fleet-agent:v0.17.0")

	err := CheckImagesExist(registry.host(), []ChartImage{
		fleetImage(),
		{Image: "rancher/fleet-agent:v0.17.0", Sources: []string{"fleet-agent:111.0.1"}},
	})

	assert.NoError(t, err)
}

func TestCheckImagesExistFindsAnImageThatTheRegistryDoesNotHave(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)
	registry.has("rancher/fleet:v0.17.0")

	err := CheckImagesExist(registry.host(), []ChartImage{
		fleetImage(),
		{Image: "rancher/cluster-api-controller:v1.14.4_turtles.1", Sources: []string{"rancher-turtles:111.0.0+up0.28.0-rc.7"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/cluster-api-controller:v1.14.4_turtles.1")
	assert.Contains(t, err.Error(), "rancher-turtles:111.0.0+up0.28.0-rc.7")
	assert.Contains(t, err.Error(), registry.host())
	assert.NotContains(t, err.Error(), "rancher/fleet:v0.17.0", "an image that is there is not reported")
}

func TestCheckImagesExistReportsEveryImageThatIsMissing(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)

	err := CheckImagesExist(registry.host(), []ChartImage{
		fleetImage(),
		{Image: "rancher/rancher-webhook:v0.12.1", Sources: []string{"rancher-webhook:111.0.1"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/fleet:v0.17.0")
	assert.Contains(t, err.Error(), "rancher/rancher-webhook:v0.12.1")
}

func TestCheckImagesExistAsksWithTheTokenOfAnAnonymousClient(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, true)
	registry.has("rancher/fleet:v0.17.0")

	err := CheckImagesExist(registry.host(), []ChartImage{fleetImage()})

	require.NoError(t, err)
	assert.Positive(t, registry.tokens, "the registry asked for a token")
}

func TestCheckImagesExistRetriesWhenTheRegistryFails(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)
	registry.has("rancher/fleet:v0.17.0", http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusOK)

	err := CheckImagesExist(registry.host(), []ChartImage{fleetImage()})

	require.NoError(t, err)
	assert.Equal(t, 3, registry.requestsFor("rancher/fleet:v0.17.0"))
}

func TestCheckImagesExistGivesUpAfterFiveAttempts(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)
	registry.has("rancher/fleet:v0.17.0", http.StatusServiceUnavailable)

	err := CheckImagesExist(registry.host(), []ChartImage{fleetImage()})

	require.Error(t, err)
	assert.Equal(t, 5, registry.requestsFor("rancher/fleet:v0.17.0"))
	assert.Contains(t, err.Error(), "rancher/fleet:v0.17.0")
	assert.NotContains(t, err.Error(), "not in", "an image that the registry could not be asked for is not said to be missing")
}

func TestCheckImagesExistDoesNotRetryAnImageThatIsMissing(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)

	err := CheckImagesExist(registry.host(), []ChartImage{fleetImage()})

	require.Error(t, err)
	assert.Equal(t, 1, registry.requestsFor("rancher/fleet:v0.17.0"))
	assert.Contains(t, err.Error(), "not in")
}

func TestCheckImagesExistFailsWhenTheRegistryDoesNotLetItAsk(t *testing.T) {
	useFakeRegistries(t)
	registry := newFakeRegistry(t, false)
	registry.has("rancher/fleet:v0.17.0", http.StatusForbidden)

	err := CheckImagesExist(registry.host(), []ChartImage{fleetImage()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/fleet:v0.17.0")
	assert.NotContains(t, err.Error(), "not in")
}

func TestImagesOfChartsTakesTheImagesWithAChartAsSource(t *testing.T) {
	lines := []string{
		"rancher/fleet:v0.17.0 fleet-agent:111.0.1,fleet:111.0.1",
		"rancher/rancher:v2.16.0 rancher",
		"rancher/mirrored-pause:3.6 k3sUpgrade,rke2All",
		"rancher/kuberlr-kubectl:v9.0.4 rancher,system-upgrade-controller:111.0.1",
		"rancher/turtles:v0.28.0-rc.7 rancher-turtles:111.0.0+up0.28.0-rc.7",
	}

	assert.Equal(t, []ChartImage{
		{Image: "rancher/fleet:v0.17.0", Sources: []string{"fleet-agent:111.0.1", "fleet:111.0.1"}},
		{Image: "rancher/kuberlr-kubectl:v9.0.4", Sources: []string{"system-upgrade-controller:111.0.1"}},
		{Image: "rancher/turtles:v0.28.0-rc.7", Sources: []string{"rancher-turtles:111.0.0+up0.28.0-rc.7"}},
	}, ImagesOfCharts(lines))
}

func TestRegistryOfAssetsImageIsStagingForADevImage(t *testing.T) {
	assert.Equal(t, "stgregistry.suse.com", RegistryOfAssetsImage("rancher/rancher-assets:v2.16-20261113T2059Z-dev"))
}

func TestRegistryOfAssetsImageIsProductionForAnyOtherImage(t *testing.T) {
	assert.Equal(t, "registry.suse.com", RegistryOfAssetsImage("rancher/rancher-assets:v2.16-20261118T1616Z"))
}
