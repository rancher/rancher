package utils

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
)

type customTargetTestWrap struct {
	*v3.CustomTargetConfig
}

func (w *customTargetTestWrap) TestReachable(ctx context.Context, dial dialer.Dialer, includeSendTestLog bool) error {
	return nil
}
