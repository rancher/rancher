//
// CODE GENERATED AUTOMATICALLY WITH github.com/kelveny/mockcompose
// THIS FILE SHOULD NOT BE EDITED BY HAND
//
package rancher

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/rest"
)

type mockClusterOperator struct {
	mock.Mock
}

func (m *mockClusterOperator) GenerateSAToken(restConfig *rest.Config) (string, error) {

	_mc_ret := m.Called(restConfig)

	var _r0 string

	if _rfn, ok := _mc_ret.Get(0).(func(*rest.Config) string); ok {
		_r0 = _rfn(restConfig)
	} else {
		if _mc_ret.Get(0) != nil {
			_r0 = _mc_ret.Get(0).(string)
		}
	}

	var _r1 error

	if _rfn, ok := _mc_ret.Get(1).(func(*rest.Config) error); ok {
		_r1 = _rfn(restConfig)
	} else {
		_r1 = _mc_ret.Error(1)
	}

	return _r0, _r1

}
