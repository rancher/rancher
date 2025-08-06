package types

import "context"

type ResourceDeployer interface {
	Ensure(ctx context.Context, labels map[string]string) error
}
