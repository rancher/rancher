package types

import "context"

const (
	// StringType is the type for string flag
	StringType = "string"
	// BoolType is the type for bool flag
	BoolType = "bool"
	// IntType is the type for int flag
	IntType = "int"
	// StringSliceType is the type for stringSlice flag
	StringSliceType = "stringSlice"
)

// Driver defines the interface that each driver plugin should implement
type Driver interface {
	// GetDriverCreateOptions returns cli flags that are used in create
	GetDriverCreateOptions(ctx context.Context) (*DriverFlags, error)

	// GetDriverUpdateOptions returns cli flags that are used in update
	GetDriverUpdateOptions(ctx context.Context) (*DriverFlags, error)

	// Create creates the cluster
	Create(ctx context.Context, opts *DriverOptions) (*ClusterInfo, error)

	// Update updates the cluster
	Update(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions) (*ClusterInfo, error)

	// PostCheck does post action after provisioning
	PostCheck(ctx context.Context, clusterInfo *ClusterInfo) (*ClusterInfo, error)

	// Remove removes the cluster
	Remove(ctx context.Context, clusterInfo *ClusterInfo) error
}
