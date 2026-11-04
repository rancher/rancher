package clustermanager

import (
	"sync"
	"testing"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAccessControl is an access control that only tells instances apart.
type fakeAccessControl struct {
	types.AccessControl
}

// newAccessControlTestManager returns a manager with a record for the cluster c-m-test, and a request
// for that cluster.
func newAccessControlTestManager() (*Manager, *record, *types.APIContext) {
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m := &Manager{
		accessControl: &fakeAccessControl{},
		clusterLister: &fakes.ClusterListerMock{
			GetFunc: func(_, name string) (*apimgmtv3.Cluster, error) {
				return r.clusterRec.DeepCopy(), nil
			},
		},
	}
	m.controllers.Store(r.clusterRec.UID, r)
	apiContext := &types.APIContext{SubContext: map[string]string{"/v3/schemas/cluster": "c-m-test"}}
	return m, r, apiContext
}

func TestAccessControlIsUnavailableUntilTheControllersCreateIt(t *testing.T) {
	m, _, apiContext := newAccessControlTestManager()

	_, err := m.AccessControl(apiContext, config.UserStorageContext)

	var apiErr *httperror.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, httperror.ClusterUnavailable, apiErr.Code)
}

func TestAccessControlOfTheCluster(t *testing.T) {
	m, r, apiContext := newAccessControlTestManager()
	accessControl := &fakeAccessControl{}
	r.setAccessControl(accessControl)

	got, err := m.AccessControl(apiContext, config.UserStorageContext)

	require.NoError(t, err)
	assert.Same(t, accessControl, got)
}

func TestAccessControlOutsideOfAClusterIsTheManagers(t *testing.T) {
	m, _, _ := newAccessControlTestManager()

	got, err := m.AccessControl(nil, config.UserStorageContext)

	require.NoError(t, err)
	assert.Same(t, m.accessControl, got)
}

func TestAccessControlCanBeReadWhileTheControllersCreateIt(t *testing.T) {
	// The access control is created by the start goroutine while requests are already being served.
	// The race detector fails this test if they aren't synchronized.
	m, r, apiContext := newAccessControlTestManager()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			r.setAccessControl(&fakeAccessControl{})
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			_, _ = m.AccessControl(apiContext, config.UserStorageContext)
		}
	}()
	wg.Wait()
}
