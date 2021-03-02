package utils

import (
	"context"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/types/config/dialer"
)

type customTargetTestWrap struct {
	*v32.CustomTargetConfig
}

func (w *customTargetTestWrap) TestReachable(ctx context.Context, dial dialer.Dialer, includeSendTestLog bool) error {
	return nil
}
