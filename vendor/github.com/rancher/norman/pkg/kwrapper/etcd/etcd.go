// +build !linux

package etcd

import (
	"context"
)

func RunETCD(ctx context.Context, dataDir string) ([]string, error) {
	return nil, nil
}
