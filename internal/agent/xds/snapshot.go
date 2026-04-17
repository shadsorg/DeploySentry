package xds

import (
	"fmt"
	"sort"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	routeConfigName = "local_route"
	listenerName    = "local_listener"
)

// HeaderOverride routes requests with a matching header to a specific upstream.
type HeaderOverride struct {
	Header   string
	Value    string
	Upstream string
}

// SnapshotOptions configures optional behaviors for snapshot building.
type SnapshotOptions struct {
	HeaderOverrides []HeaderOverride
}

// BuildSnapshot creates an Envoy xDS snapshot with weighted clusters.
func BuildSnapshot(version string, upstreams map[string]string, weights map[string]uint32, listenPort uint32) (*cachev3.Snapshot, error) {
	return BuildSnapshotWithOptions(version, upstreams, weights, listenPort, SnapshotOptions{})
}

// BuildSnapshotWithOptions creates a complete Envoy xDS snapshot with weighted
// clusters and optional header-based route overrides.
func BuildSnapshotWithOptions(version string, upstreams map[string]string, weights map[string]uint32, listenPort uint32, opts SnapshotOptions) (*cachev3.Snapshot, error) {
	clusters := makeClusters(upstreams)
	routeConfig := makeRouteConfig(weights, opts)
	listeners, err := makeListeners(listenPort)
	if err != nil {
		return nil, fmt.Errorf("building listener: %w", err)
	}

	snap, err := cachev3.NewSnapshot(version, map[resource.Type][]types.Resource{
		resource.ClusterType:  clusters,
		resource.RouteType:    {routeConfig},
		resource.ListenerType: listeners,
	})
	if err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}

	if err := snap.Consistent(); err != nil {
		return nil, fmt.Errorf("inconsistent snapshot: %w", err)
	}

	return snap, nil
}

func makeClusters(upstreams map[string]string) []types.Resource {
	names := sortedKeys(upstreams)
	clusters := make([]types.Resource, 0, len(upstreams))
	for _, name := range names {
		host, port := splitHostPort(upstreams[name])
		c := &cluster.Cluster{
			Name:                 name,
			ConnectTimeout:       durationpb.New(5 * time.Second),
			ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STRICT_DNS},
			LbPolicy:             cluster.Cluster_ROUND_ROBIN,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: name,
				Endpoints: []*endpoint.LocalityLbEndpoints{{
					LbEndpoints: []*endpoint.LbEndpoint{{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address: &core.Address{
									Address: &core.Address_SocketAddress{
										SocketAddress: &core.SocketAddress{
											Address: host,
											PortSpecifier: &core.SocketAddress_PortValue{
												PortValue: port,
											},
										},
									},
								},
							},
						},
					}},
				}},
			},
		}
		clusters = append(clusters, c)
	}
	return clusters
}

func makeRouteConfig(weights map[string]uint32, opts SnapshotOptions) *route.RouteConfiguration {
	var routes []*route.Route

	// Header override routes come first (first-match wins).
	for _, ho := range opts.HeaderOverrides {
		routes = append(routes, &route.Route{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
				Headers: []*route.HeaderMatcher{{
					Name: ho.Header,
					HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
						ExactMatch: ho.Value,
					},
				}},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: ho.Upstream,
					},
				},
			},
		})
	}

	// Default weighted cluster route.
	routes = append(routes, &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_WeightedClusters{
					WeightedClusters: makeWeightedClusters(weights),
				},
			},
		},
	})

	return &route.RouteConfiguration{
		Name: routeConfigName,
		VirtualHosts: []*route.VirtualHost{{
			Name:    "local_service",
			Domains: []string{"*"},
			Routes:  routes,
		}},
	}
}

func makeWeightedClusters(weights map[string]uint32) *route.WeightedCluster {
	names := sortedKeys(weights)
	wcs := make([]*route.WeightedCluster_ClusterWeight, 0, len(weights))
	for _, name := range names {
		wcs = append(wcs, &route.WeightedCluster_ClusterWeight{
			Name:   name,
			Weight: wrapperspb.UInt32(weights[name]),
		})
	}
	return &route.WeightedCluster{Clusters: wcs}
}

func makeListeners(port uint32) ([]types.Resource, error) {
	routerAny, err := anypb.New(&router.Router{})
	if err != nil {
		return nil, fmt.Errorf("marshaling router filter: %w", err)
	}

	hcmConfig := &hcm.HttpConnectionManager{
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource: &core.ConfigSource{
					ConfigSourceSpecifier: &core.ConfigSource_Ads{
						Ads: &core.AggregatedConfigSource{},
					},
				},
				RouteConfigName: routeConfigName,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: "envoy.filters.http.router",
			ConfigType: &hcm.HttpFilter_TypedConfig{
				TypedConfig: routerAny,
			},
		}},
	}

	hcmAny, err := anypb.New(hcmConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling HCM: %w", err)
	}

	l := &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: hcmAny,
				},
			}},
		}},
	}
	return []types.Resource{l}, nil
}

// splitHostPort splits "host:port" into host string and port uint32.
func splitHostPort(addr string) (string, uint32) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			var port uint32
			for _, c := range addr[i+1:] {
				port = port*10 + uint32(c-'0')
			}
			return addr[:i], port
		}
	}
	return addr, 80
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
