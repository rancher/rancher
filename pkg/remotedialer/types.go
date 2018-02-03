package remotedialer

import (
	"net"
	"time"
)

var (
	PingWaitDuration  = time.Duration(10 * time.Second)
	PingWriteInterval = time.Duration(5 * time.Second)
	MaxRead           = 8192
)

type Dialer func(proto, address string) (net.Conn, error)
