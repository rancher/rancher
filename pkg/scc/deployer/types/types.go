package types

import "context"

type ResourceDeployer interface {
	HasResource() (bool, error)
	Ensure(ctx context.Context, labels map[string]string) error
}
