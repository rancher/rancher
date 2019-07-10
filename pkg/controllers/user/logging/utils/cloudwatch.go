package utils

import (
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

type cloudWatchTestWrap struct {
	*v3.CloudWatchConfig
}

func (cloudWatchTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	// TODO: Test Reachable
	return nil
}
