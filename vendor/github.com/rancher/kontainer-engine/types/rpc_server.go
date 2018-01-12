package types

import (
	"net"

	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

const (
	listenAddr = "127.0.0.1:"
)

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
	return s.driver.GetDriverCreateOptions(ctx)
}

// GetDriverUpdateOptions implements grpc method
func (s *GrpcServer) GetDriverUpdateOptions(ctx context.Context, in *Empty) (*DriverFlags, error) {
	return s.driver.GetDriverUpdateOptions(ctx)
}

// Create implements grpc method
func (s *GrpcServer) Create(ctx context.Context, opts *DriverOptions) (*ClusterInfo, error) {
	return s.driver.Create(getCtx(ctx), opts)
}

// Update implements grpc method
func (s *GrpcServer) Update(ctx context.Context, update *UpdateRequest) (*ClusterInfo, error) {
	return s.driver.Update(getCtx(ctx), update.ClusterInfo, update.DriverOptions)
}

func (s *GrpcServer) PostCheck(ctx context.Context, clusterInfo *ClusterInfo) (*ClusterInfo, error) {
	return s.driver.PostCheck(getCtx(ctx), clusterInfo)
}

// Remove implements grpc method
func (s *GrpcServer) Remove(ctx context.Context, clusterInfo *ClusterInfo) (*Empty, error) {
	return &Empty{}, s.driver.Remove(getCtx(ctx), clusterInfo)
}

func getCtx(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	logID, ok := md["log-id"]
	if !ok || len(logID) == 0 {
		return ctx
	}
	logger := logstream.GetLogStream(logID[0])
	if logger == nil {
		return ctx
	}
	return log.SetLogger(ctx, logger)
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
