package drivers

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// NewClient creates a grpc client for a driver plugin
func NewClient(driverName string, addr string) (*GrpcClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	c := NewDriverClient(conn)
	return &GrpcClient{
		client:     c,
		driverName: driverName,
	}, nil
}

// GrpcClient defines the grpc client struct
type GrpcClient struct {
	client     DriverClient
	driverName string
}

// Create call grpc create
func (rpc *GrpcClient) Create() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	_, err := rpc.client.Create(ctx, &Empty{})
	return err
}

// Update call grpc update
func (rpc *GrpcClient) Update() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	_, err := rpc.client.Update(ctx, &Empty{})
	return err
}

// Get call grpc get
func (rpc *GrpcClient) Get() ClusterInfo {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	info, err := rpc.client.Get(ctx, &Empty{})
	if err != nil {
		return ClusterInfo{}
	}
	return *info
}

func (rpc *GrpcClient) PostCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	if _, err := rpc.client.PostCheck(ctx, &Empty{}); err != nil {
		return err
	}
	return nil
}

// Remove call grpc remove
func (rpc *GrpcClient) Remove() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	_, err := rpc.client.Remove(ctx, &Empty{})
	return err
}

// GetDriverCreateOptions call grpc getDriverCreateOptions
func (rpc *GrpcClient) GetDriverCreateOptions() (DriverFlags, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	flags, err := rpc.client.GetDriverCreateOptions(ctx, &Empty{})
	if err != nil {
		return DriverFlags{}, err
	}
	return *flags, nil
}

// GetDriverUpdateOptions call grpc getDriverUpdateOptions
func (rpc *GrpcClient) GetDriverUpdateOptions() (DriverFlags, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	flags, err := rpc.client.GetDriverUpdateOptions(ctx, &Empty{})
	if err != nil {
		return DriverFlags{}, err
	}
	return *flags, nil
}

// SetDriverOptions set the driver options
func (rpc *GrpcClient) SetDriverOptions(options DriverOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	_, err := rpc.client.SetDriverOptions(ctx, &options)
	return err
}

// DriverName returns the driver name
func (rpc *GrpcClient) DriverName() string {
	return rpc.driverName
}
