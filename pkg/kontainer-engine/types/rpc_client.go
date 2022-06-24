package types

import (
	"context"

	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// NewClient creates a grpc client for a driver plugin
func NewClient(driverName string, addr string) (CloseableDriver, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	c := NewDriverClient(conn)
	return &grpcClient{
		client:     c,
		driverName: driverName,
		conn:       conn,
	}, nil
}

// grpcClient defines the grpc client struct
type grpcClient struct {
	client     DriverClient
	driverName string
	conn       *grpc.ClientConn
}

// Create call grpc create
func (rpc *grpcClient) Create(ctx context.Context, opts *DriverOptions, clusterInfo *ClusterInfo) (*ClusterInfo, error) {
	o, err := rpc.client.Create(ctx, &CreateRequest{
		DriverOptions: opts,
		ClusterInfo:   clusterInfo,
	})
	err = handlErr(err)
	if err == nil && o.CreateError != "" {
		err = errors.New(o.CreateError)
	}
	return o, err
}

// Update call grpc update
func (rpc *grpcClient) Update(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions) (*ClusterInfo, error) {
	o, err := rpc.client.Update(ctx, &UpdateRequest{
		ClusterInfo:   clusterInfo,
		DriverOptions: opts,
	})
	return o, handlErr(err)
}

func (rpc *grpcClient) PostCheck(ctx context.Context, clusterInfo *ClusterInfo) (*ClusterInfo, error) {
	o, err := rpc.client.PostCheck(ctx, clusterInfo)
	return o, handlErr(err)
}

// Remove call grpc remove
func (rpc *grpcClient) Remove(ctx context.Context, clusterInfo *ClusterInfo) error {
	_, err := rpc.client.Remove(ctx, clusterInfo)
	return handlErr(err)
}

// GetDriverCreateOptions call grpc getDriverCreateOptions
func (rpc *grpcClient) GetDriverCreateOptions(ctx context.Context) (*DriverFlags, error) {
	o, err := rpc.client.GetDriverCreateOptions(ctx, &Empty{})
	return o, handlErr(err)
}

// GetDriverUpdateOptions call grpc getDriverUpdateOptions
func (rpc *grpcClient) GetDriverUpdateOptions(ctx context.Context) (*DriverFlags, error) {
	o, err := rpc.client.GetDriverUpdateOptions(ctx, &Empty{})
	return o, handlErr(err)
}

func (rpc *grpcClient) GetVersion(ctx context.Context, info *ClusterInfo) (*KubernetesVersion, error) {
	version, err := rpc.client.GetVersion(ctx, info)
	return version, handlErr(err)
}

func (rpc *grpcClient) SetVersion(ctx context.Context, info *ClusterInfo, version *KubernetesVersion) error {
	_, err := rpc.client.SetVersion(ctx, &SetVersionRequest{Info: info, Version: version})
	return handlErr(err)
}

func (rpc *grpcClient) GetClusterSize(ctx context.Context, info *ClusterInfo) (*NodeCount, error) {
	size, err := rpc.client.GetNodeCount(ctx, info)
	return size, handlErr(err)
}

func (rpc *grpcClient) SetClusterSize(ctx context.Context, info *ClusterInfo, count *NodeCount) error {
	_, err := rpc.client.SetNodeCount(ctx, &SetNodeCountRequest{Info: info, Count: count})
	return handlErr(err)
}

func (rpc *grpcClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	return rpc.client.GetCapabilities(ctx, &Empty{})
}

func (rpc *grpcClient) GetK8SCapabilities(ctx context.Context, opts *DriverOptions) (*K8SCapabilities, error) {
	capabilities, err := rpc.client.GetK8SCapabilities(ctx, opts)
	return capabilities, handlErr(err)
}

func (rpc *grpcClient) Close() error {
	return rpc.conn.Close()
}

func (rpc *grpcClient) ETCDSave(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions, snapshotName string) error {
	_, err := rpc.client.ETCDSave(ctx, &SaveETCDSnapshotRequest{Info: clusterInfo, SnapshotName: snapshotName, DriverOptions: opts})
	return handlErr(err)
}

func (rpc *grpcClient) ETCDRestore(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions, snapshotName string) (*ClusterInfo, error) {
	o, err := rpc.client.ETCDRestore(ctx, &RestoreETCDSnapshotRequest{Info: clusterInfo, SnapshotName: snapshotName, DriverOptions: opts})
	return o, handlErr(err)
}

func (rpc *grpcClient) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions, snapshotName string) error {
	_, err := rpc.client.ETCDRemoveSnapshot(ctx, &RemoveETCDSnapshotRequest{Info: clusterInfo, SnapshotName: snapshotName, DriverOptions: opts})
	return handlErr(err)
}

func (rpc *grpcClient) RemoveLegacyServiceAccount(ctx context.Context, info *ClusterInfo) error {
	_, err := rpc.client.RemoveLegacyServiceAccount(ctx, info)
	return handlErr(err)
}

func handlErr(err error) error {
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unknown && st.Message() != "" {
			return errors.New(st.Message())
		}
	}
	return err
}
