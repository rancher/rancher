package helm

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"io/ioutil"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	testing2 "k8s.io/kubectl/pkg/cmd/testing"
)

func TestListReleases(t *testing.T) {
	t.Parallel()
	asserts := assert.New(t)
	type testInput struct {
		name             string
		stateMask        action.ListStates
		releases         []*release.Release
		restClientGetter genericclioptions.RESTClientGetter
		namespace        string
		runAction        func(l *action.List) ([]*release.Release, error)
	}

	type testCase struct {
		name  string
		input testInput
		skip  bool
		fails bool
	}

	testRelease := []*release.Release{{Name: "test", Version: 1, Info: &release.Info{Status: release.StatusPendingInstall}}}

	testCases := []testCase{{
		name: "name and stateMask matches",
		input: testInput{
			name:             "test",
			stateMask:        action.ListPendingInstall,
			releases:         testRelease,
			restClientGetter: testing2.NewTestFactory(),
			namespace:        "",
		},
		skip: false,
	}, {
		name: "name does not match",
		input: testInput{
			name:             "random",
			stateMask:        action.ListPendingInstall,
			restClientGetter: testing2.NewTestFactory(),
			namespace:        "",
			releases:         testRelease,
		},
		skip: false,
	}, {
		name: "stateMask does not match",
		input: testInput{
			name:             "test",
			stateMask:        action.ListDeployed,
			releases:         testRelease,
			restClientGetter: testing2.NewTestFactory(),
			namespace:        "test",
		},
		skip: false,
	}, {
		name: "Name and state does not match",
		input: testInput{
			name:             "random",
			stateMask:        action.ListPendingUpgrade,
			releases:         testRelease,
			restClientGetter: testing2.NewTestFactory(),
			namespace:        "test",
		},
		skip: false,
	}, {
		name: "Run action modifies the result of list",
		input: testInput{
			name:             "test",
			stateMask:        action.ListPendingInstall,
			releases:         testRelease,
			restClientGetter: testing2.NewTestFactory(),
			namespace:        "",
			runAction: func(l *action.List) ([]*release.Release, error) {
				r, e := l.Run()
				r[0].Manifest = "random stuff"
				return r, e
			},
		},
		skip:  false,
		fails: true,
	}}
	for _, test := range testCases {
		if test.skip {
			continue
		}
		r, _ := registry.NewClient()

		mockCfg := &action.Configuration{
			Releases:       storage.Init(driver.NewMemory()),
			KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: ioutil.Discard}},
			Capabilities:   chartutil.DefaultCapabilities,
			RegistryClient: r,
			Log: func(format string, v ...interface{}) {
				t.Helper()
			},
		}
		for _, r := range test.input.releases {
			asserts.NoError(mockCfg.Releases.Create(r), test.name)
		}
		var originalReleases []*release.Release
		var originalErr error
		client := Client{
			restClientGetter: test.input.restClientGetter,
			actRun: func(list *action.List) ([]*release.Release, error) {
				//filter and stateMask should be set
				asserts.Equal("^"+test.input.name+"$", list.Filter, test.name)
				asserts.Equal(test.input.stateMask, list.StateMask, test.name)

				r, e := list.Run()
				err := deepCopy(&originalReleases, r)
				asserts.NoError(err)
				err = deepCopy(&originalErr, &e)
				asserts.NoError(err)
				if test.input.runAction != nil {
					return test.input.runAction(list)
				}
				return runAction(list)
			},
			newList: func(c *action.Configuration) *action.List {
				return action.NewList(mockCfg)
			},
		}

		resp, err := client.ListReleases(test.input.namespace, test.input.name, test.input.stateMask)
		//response should be the same as list.Run()
		asserts.True(reflect.DeepEqual(err, originalErr), test.name)
		if len(resp) == 0 {
			asserts.Equal(len(originalReleases), len(resp), test.name)
		} else if !test.fails {
			asserts.True(reflect.DeepEqual(resp, originalReleases), test.name)
		} else {
			asserts.False(reflect.DeepEqual(resp, originalReleases), test.name)
		}
	}

}

func deepCopy(dst, src interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}
