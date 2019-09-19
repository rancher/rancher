package remotedialer

import (
	"time"
)

var (
	PingWaitDuration  = time.Duration(60 * time.Second)
	PingWriteInterval = time.Duration(5 * time.Second)
	MaxRead           = 8192
)
