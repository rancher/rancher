// +build !linux

package k8s

import (
	"context"

	"k8s.io/client-go/rest"
)

func getEmbedded(ctx context.Context) (bool, context.Context, *rest.Config, error) {
	return false, ctx, nil, fmt.Error("embedded only supported on linux")

}
