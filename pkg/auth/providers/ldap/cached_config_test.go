package ldap

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigCache_GetConfigMap(t *testing.T) {
	t.Parallel()
	remoteConf := &fakeRemoteConfig{}
	p := &ldapProvider{
		providerName:    "fake-provider",
		configMapGetter: NewCachedConfig(remoteConf, time.Minute*5),
	}
	// 1st call will be a cache miss due to first run
	got, err := p.configMapGetter.GetConfigMap(p.providerName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if remoteConf.nCalls != 1 {
		t.Fatalf("ncalls should be one, got %d", remoteConf.nCalls)
	}
	want := map[string]interface{}{
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
	if remoteConf.nCalls != 1 {
		t.Fatalf("ncalls should be one due to cache hit, got %d", remoteConf.nCalls)
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
	if remoteConf.nCalls != 2 {
		t.Fatalf("ncalls should be 2 due to cache miss related to Expire(), got %d", remoteConf.nCalls)
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
