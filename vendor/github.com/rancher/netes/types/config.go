package types

import (
	"context"
	"net"

	"github.com/rancher/netes/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type DialerContext func(context.Context, string, string) (net.Conn, error)

type DialerFactory interface {
	Dialer(cluster *v3.Cluster) (DialerContext, error)
}

type GlobalConfig struct {
	Dialect    string
	DSN        string
	CattleURL  string
	ListenAddr string

	AdmissionControllers []string
	ServiceNetCidr       string

	Lookup        cluster.Lookup
	DialerFactory DialerFactory
}

func FirstNotEmpty(left, right string) string {
	if left != "" {
		return left
	}
	return right
}

func FirstNotLenZero(left, right []string) []string {
	if len(left) > 0 {
		return left
	}
	return right
}
