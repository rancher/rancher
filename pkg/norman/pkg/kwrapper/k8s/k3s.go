//go:build !linux
// +build !linux

package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
)

func getEmbedded(ctx context.Context) (bool, clientcmd.ClientConfig, error) {
	return false, nil, fmt.Errorf("embedded only supported on linux")
}
