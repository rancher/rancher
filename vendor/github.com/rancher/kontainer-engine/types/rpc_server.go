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
	return s.driver.Create(GetCtx(ctx), opts)
}

// Update implements grpc method
func (s *GrpcServer) Update(ctx context.Context, update *UpdateRequest) (*ClusterInfo, error) {
	return s.driver.Update(GetCtx(ctx), update.ClusterInfo, update.DriverOptions)
}

func (s *GrpcServer) PostCheck(ctx context.Context, clusterInfo *ClusterInfo) (*ClusterInfo, error) {
	return s.driver.PostCheck(GetCtx(ctx), clusterInfo)
}

func (s *GrpcServer) GetVersion(ctx context.Context, clusterInfo *ClusterInfo) (*KubernetesVersion, error) {
	return s.driver.GetVersion(GetCtx(ctx), clusterInfo)
}

func (s *GrpcServer) SetVersion(ctx context.Context, request *SetVersionRequest) (*Empty, error) {
	return &Empty{}, s.driver.SetVersion(GetCtx(ctx), request.Info, request.Version)
}

func (s *GrpcServer) GetNodeCount(ctx context.Context, clusterInfo *ClusterInfo) (*NodeCount, error) {
	return s.driver.GetClusterSize(GetCtx(ctx), clusterInfo)
}

func (s *GrpcServer) SetNodeCount(ctx context.Context, request *SetNodeCountRequest) (*Empty, error) {
	return &Empty{}, s.driver.SetClusterSize(GetCtx(ctx), request.Info, request.Count)
}

// Remove implements grpc method
func (s *GrpcServer) Remove(ctx context.Context, clusterInfo *ClusterInfo) (*Empty, error) {
	return &Empty{}, s.driver.Remove(GetCtx(ctx), clusterInfo)
}

func GetCtx(ctx context.Context) context.Context {
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

func (s *GrpcServer) GetCapabilities(ctx context.Context, in *Empty) (*Capabilities, error) {
	return s.driver.GetCapabilities(ctx)
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
