package ldap

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigCache_GetConfigMap(t *testing.T) {
	t.Parallel()
	p := &ldapProvider{
		providerName:    "fake-provider",
		configMapGetter: NewCachedConfig(&fakeRemoteConfig{}, time.Minute*5),
	}
	// 1st call will be a cache miss due to first run
	got, err := p.configMapGetter.GetConfigMap(p.providerName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ncalls := got["nCalls"].(int)
	if ncalls != 0 {
		t.Fatalf("ncalls should be zero, got %d", ncalls)
	}
	want := map[string]interface{}{
		// copy nCalls, which is generated in the fake
		"nCalls":              got["nCalls"],
		"type":                "fake type",
		"enabled":             true,
		"accessMode":          "fake access mode",
		"allowedPrincipalIds": []string{"foo", "bar"},
		"servers":             []string{"server1", "server2"},
		"port":                25000,
		"metadata": map[string]interface{}{
			"name": "meta name",
		},
	}
	configMapEqual(t, got, want)
	// 2nd call will be a cache hit
	got, err = p.configMapGetter.GetConfigMap(p.providerName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ncalls = got["nCalls"].(int)
	if ncalls != 0 {
		t.Fatalf("ncalls should be zero due to cache hit, got %d", ncalls)
	}
	configMapEqual(t, got, want)
	ex := p.configMapGetter.(expirer)
	err = ex.Expire()
	if err != nil {
		t.Fatal(err)
	}
	// 3rd call will be a cache miss due to Expire() being run
	got, err = p.configMapGetter.GetConfigMap(p.providerName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want["nCalls"] = got["nCalls"]
	ncalls = got["nCalls"].(int)
	if ncalls != 1 {
		t.Fatalf("ncalls should be 1 due to cache miss related to Expire(), got %d", ncalls)
	}
	configMapEqual(t, got, want)
}

func configMapEqual(t *testing.T, got, want map[string]interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrong configMap")
		t.Logf(" got: %#v", got)
		t.Logf("want: %#v", want)
	}
}

type expirer interface {
	Expire() error
}

// fakeRemoteConfig pretends to be a RemoteConfig, without v3.AuthConfigInterface bloat.
type fakeRemoteConfig struct {
	// nCalls increments on cache miss
	nCalls int
}

func (f *fakeRemoteConfig) GetConfigMap(name string, opts metav1.GetOptions) (map[string]interface{}, error) {
	defer func() { f.nCalls++ }()
	return map[string]interface{}{
		"nCalls":              f.nCalls,
		"type":                "fake type",
		"enabled":             true,
		"accessMode":          "fake access mode",
		"allowedPrincipalIds": []string{"foo", "bar"},
		"servers":             []string{"server1", "server2"},
		"port":                25000,
		"metadata": map[string]interface{}{
			"name": "meta name",
		},
	}, nil
}
