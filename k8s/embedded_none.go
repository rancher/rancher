// +build !k8s

package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
)

func getEmbedded(ctx context.Context) (context.Context, *rest.Config, error) {
	return ctx, nil, fmt.Errorf("embedded support is not compiled in, rebuild with -tags k8s")
}
