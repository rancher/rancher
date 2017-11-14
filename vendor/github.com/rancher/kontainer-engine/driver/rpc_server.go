package drivers

import (
	"net"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	listenAddr = "127.0.0.1:"
)

// Driver defines the interface that each driver plugin should implement
type Driver interface {
	// GetDriverCreateOptions returns cli flags that are used in create
	GetDriverCreateOptions() (*DriverFlags, error)

	// GetDriverUpdateOptions returns cli flags that are used in update
	GetDriverUpdateOptions() (*DriverFlags, error)

	// SetDriverOptions set the driver options into plugin. String, bool, int and stringslice are currently four supported types.
	SetDriverOptions(driverOptions *DriverOptions) error

	// Create creates the cluster
	Create() error

	// Update updates the cluster
	Update() error

	// Get retrieve the cluster and return cluster info
	Get() (*ClusterInfo, error)

	// PostCheck does post action after provisioning
	PostCheck() error

	// Remove removes the cluster
	Remove() error
}

// GrpcServer defines the server struct
type GrpcServer struct {
	driver  Driver
	address chan string
}

// NewServer creates a grpc server for a specific plugin
func NewServer(driver Driver, addr chan string) *GrpcServer {
	return &GrpcServer{
		driver:  driver,
		address: addr,
	}
}

// GetDriverCreateOptions implements grpc method
func (s *GrpcServer) GetDriverCreateOptions(ctx context.Context, in *Empty) (*DriverFlags, error) {
	return s.driver.GetDriverCreateOptions()
}

// GetDriverUpdateOptions implements grpc method
func (s *GrpcServer) GetDriverUpdateOptions(ctx context.Context, in *Empty) (*DriverFlags, error) {
	return s.driver.GetDriverUpdateOptions()
}

// SetDriverOptions implements grpc method
func (s *GrpcServer) SetDriverOptions(ctx context.Context, in *DriverOptions) (*Empty, error) {
	return &Empty{}, s.driver.SetDriverOptions(in)
}

// Create implements grpc method
func (s *GrpcServer) Create(ctx context.Context, in *Empty) (*Empty, error) {
	return &Empty{}, s.driver.Create()
}

// Update implements grpc method
func (s *GrpcServer) Update(ctx context.Context, in *Empty) (*Empty, error) {
	return &Empty{}, s.driver.Update()
}

// Get implements grpc method
func (s *GrpcServer) Get(cont context.Context, in *Empty) (*ClusterInfo, error) {
	return s.driver.Get()
}

func (s *GrpcServer) PostCheck(cont context.Context, in *Empty) (*Empty, error) {
	return &Empty{}, s.driver.PostCheck()
}

// Remove implements grpc method
func (s *GrpcServer) Remove(ctx context.Context, in *Empty) (*Empty, error) {
	return &Empty{}, s.driver.Remove()
}

// Serve serves a grpc server
func (s *GrpcServer) Serve() {
	listen, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logrus.Fatal(err)
	}
	addr := listen.Addr().String()
	s.address <- addr
	grpcServer := grpc.NewServer()
	RegisterDriverServer(grpcServer, s)
	reflection.Register(grpcServer)
	logrus.Debugf("RPC GrpcServer listening on address %s", addr)
	if err := grpcServer.Serve(listen); err != nil {
		logrus.Fatal(err)
	}
	return
}
