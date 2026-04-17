package xds

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync/atomic"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

const nodeID = "deploysentry-envoy"

// Server is a gRPC server implementing the Envoy xDS control plane.
type Server struct {
	port          int
	cache         cachev3.SnapshotCache
	grpcServer    *grpc.Server
	configVersion atomic.Int64
}

// NewServer creates a new xDS server with a SnapshotCache.
func NewServer(port int) (*Server, error) {
	cache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, nil)
	return &Server{
		port:  port,
		cache: cache,
	}, nil
}

// Start creates a gRPC listener, registers all xDS services, and blocks
// until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("xds: listen on port %d: %w", s.port, err)
	}

	xdsSrv := serverv3.NewServer(ctx, s.cache, nil)
	s.grpcServer = grpc.NewServer()

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(s.grpcServer, xdsSrv)
	clusterservice.RegisterClusterDiscoveryServiceServer(s.grpcServer, xdsSrv)
	endpointservice.RegisterEndpointDiscoveryServiceServer(s.grpcServer, xdsSrv)
	listenerservice.RegisterListenerDiscoveryServiceServer(s.grpcServer, xdsSrv)
	routeservice.RegisterRouteDiscoveryServiceServer(s.grpcServer, xdsSrv)

	go func() {
		<-ctx.Done()
		s.grpcServer.GracefulStop()
	}()

	log.Printf("xds: serving on %s", lis.Addr())
	return s.grpcServer.Serve(lis)
}

// UpdateWeights builds a new xDS snapshot with the given upstream weights and
// pushes it into the cache.
func (s *Server) UpdateWeights(upstreams map[string]string, weights map[string]uint32, listenPort uint32, opts SnapshotOptions) error {
	ver := s.configVersion.Add(1)
	version := fmt.Sprintf("%d", ver)

	snap, err := BuildSnapshotWithOptions(version, upstreams, weights, listenPort, opts)
	if err != nil {
		return fmt.Errorf("xds: build snapshot v%s: %w", version, err)
	}

	if err := s.cache.SetSnapshot(context.Background(), nodeID, snap); err != nil {
		return fmt.Errorf("xds: set snapshot v%s: %w", version, err)
	}

	log.Printf("xds: pushed snapshot v%s (%d upstreams)", version, len(upstreams))
	return nil
}

// ConfigVersion returns the current snapshot version counter.
func (s *Server) ConfigVersion() int64 {
	return s.configVersion.Load()
}
