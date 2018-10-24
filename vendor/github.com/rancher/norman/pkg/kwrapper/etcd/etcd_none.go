// +build no_etcd

package etcd

import (
	"context"
)

func RunETCD(ctx context.Context) ([]string, error) {
	return nil, nil
}
