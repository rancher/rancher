package ratelimit

import (
	"context"

	"k8s.io/client-go/util/flowcontrol"
)

var (
	None = flowcontrol.RateLimiter((*none)(nil))
)

type none struct{}

func (*none) TryAccept() bool              { return true }
func (*none) Stop()                        {}
func (*none) Accept()                      {}
func (*none) QPS() float32                 { return 1 }
func (*none) Wait(_ context.Context) error { return nil }
