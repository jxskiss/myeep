package myxds

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoytypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	envoyserver "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"

	"github.com/golang/protobuf/ptypes"
	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/easy"
	"github.com/jxskiss/gopkg/exp/zlog"
	"github.com/spf13/cast"
	"google.golang.org/grpc"

	"github.com/jxskiss/myxdsdemo/myxds/provider"
	"github.com/jxskiss/myxdsdemo/pkg/model"
)

const grpcMaxConcurrentStreams = 100000

type ConfigState struct {
	DomainGroups    []*model.DomainGroup
	ServiceGroups   []*model.ServiceGroup
	StaticUpstreams []*model.StaticUpstream
}

type clusterNameHash struct{}

func (_ clusterNameHash) ID(node *core.Node) string {
	if node == nil {
		return ""
	}
	return node.Cluster
}

type Manager struct {
	prov      provider.Provider
	cache     envoycache.SnapshotCache
	callbacks *Callbacks
}

func NewManager(prov provider.Provider) *Manager {
	cache := envoycache.NewSnapshotCache(true, clusterNameHash{}, zlog.S())
	return &Manager{
		prov:      prov,
		cache:     cache,
		callbacks: &Callbacks{},
	}
}

func (p *Manager) Watch() error {
	ctx := context.Background()
	state, err := p.readConfigState(ctx)
	if err != nil {
		return err
	}
	snap, err := p.createSnapshot(ctx, state)
	if err != nil {
		return err
	}

	clusterName := "infra.l7lb.myxds_demo_envoy" // FIXME
	err = p.cache.SetSnapshot(ctx, clusterName, *snap)
	if err != nil {
		return errors.WithMessage(err, "failed set snapshot")
	}

	// TODO: watch implementation

	return nil
}

func (p *Manager) RunServer(ctx context.Context, addr string) error {

	var grpcOpts []grpc.ServerOption
	grpcOpts = append(grpcOpts, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOpts...)

	// register xDS services
	serv := envoyserver.NewServer(ctx, p.cache, p.callbacks)
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, serv)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, serv)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, serv)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, serv)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, serv)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, serv)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcServer, serv)

	zlog.Infof("run ads server listening: %v", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.AddStack(err)
	}

	go func() {
		grpcErr := grpcServer.Serve(lis)
		if grpcErr != nil {
			zlog.Fatalf("failed run grpc server, err= %v", err)
		}
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()
	return nil
}

func (p *Manager) readConfigState(ctx context.Context) (*ConfigState, error) {
	domainGroups, err := p.prov.ListDomainGroups(ctx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	serviceGroups, err := p.prov.ListServiceGroups(ctx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	staticUpstreams, err := p.prov.ListStaticUpstreams(ctx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return &ConfigState{
		DomainGroups:    domainGroups,
		ServiceGroups:   serviceGroups,
		StaticUpstreams: staticUpstreams,
	}, nil
}

func (p *Manager) createSnapshot(ctx context.Context, state *ConfigState) (*envoycache.Snapshot, error) {
	listener_, err := p.createListener(ctx)
	if err != nil {
		return nil, err
	}
	virtualHosts, err := p.createVirtualHosts(ctx, state)
	if err != nil {
		return nil, err
	}
	clusters, err := p.createClusters(ctx, state)
	if err != nil {
		return nil, err
	}
	endpoints, err := p.createEndpoints(ctx, state)
	if err != nil {
		return nil, err
	}

	virtualHostList := easy.MapValues(virtualHosts).([]*route.VirtualHost)
	sort.Slice(virtualHostList, func(i, j int) bool {
		return virtualHostList[i].Name < virtualHostList[j].Name
	})

	rte := &route.RouteConfiguration{
		Name:         "should_be_envoy_cluster_name",
		VirtualHosts: virtualHostList,
	}
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: rte,
		},
		HttpFilters: []*hcm.HttpFilter{
			{
				Name: wellknown.Router,
			},
		},
	}
	pbst, err := ptypes.MarshalAny(manager)
	if err != nil {
		return nil, err
	}
	listener_.FilterChains = []*listener.FilterChain{
		{
			Filters: []*listener.Filter{
				{
					Name: wellknown.HTTPConnectionManager,
					ConfigType: &listener.Filter_TypedConfig{
						TypedConfig: pbst,
					},
				},
			},
		},
	}

	version := fmt.Sprint(time.Now().UnixNano())
	snap, err := envoycache.NewSnapshot(version, map[resource.Type][]envoytypes.Resource{
		resource.EndpointType: endpoints,
		resource.ClusterType:  clusters,
		resource.ListenerType: {listener_},
	})
	if err != nil {
		return nil, errors.WithMessage(err, "failed create snapshot")
	}
	if err = snap.Consistent(); err != nil {
		return nil, errors.WithMessage(err, "snapshot is not consistent")
	}
	return &snap, nil
}

func (p *Manager) createListener(ctx context.Context) (*listener.Listener, error) {
	listener_ := &listener.Listener{
		Name: "listener_0",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: 10000,
					},
				},
			},
		},
	}
	return listener_, nil
}

func (p *Manager) createVirtualHosts(ctx context.Context, state *ConfigState) (map[string]*route.VirtualHost, error) {
	result := make(map[string]*route.VirtualHost, len(state.DomainGroups))
	domainGroupMap := easy.ToMap(state.DomainGroups, "Name").(map[string]*model.DomainGroup)
	extraRoutes := make(map[string][]*route.Route)
	for _, group := range state.ServiceGroups {
		domainGroup := domainGroupMap[group.DefaultDomainGroup]
		if domainGroup == nil {
			return nil, errors.Errorf("domain group %v not exists", group.DefaultDomainGroup)
		}

		for _, staticSvc := range group.StaticServices {
			clusterName := staticSvc.Upstream
			for _, location := range staticSvc.Locations {
				route_, err := makeRoute(location)
				if err != nil {
					return nil, errors.WithMessage(err, "failed make route")
				}

				route_.Action = &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
					},
				}
				addRouteToVirtualHost(route_, result, domainGroup)

				if len(location.ExtraDomainGroups) > 0 {
					for _, extraDG := range location.ExtraDomainGroups {
						dg := domainGroupMap[extraDG]
						if dg == nil {
							return nil, errors.Errorf("domain group %v not found", extraDG)
						}

						copyRoute := &route.Route{}
						copyMessage(copyRoute, route_)
						extraRoutes[dg.Name] = append(extraRoutes[dg.Name], copyRoute)
					}
				}
			}
		}

		for _, svc := range group.Services {
			clusterName := svc.Name
			for _, location := range svc.Locations {
				route_, err := makeRoute(location)
				if err != nil {
					return nil, errors.WithMessage(err, "failed make route")
				}

				route_.Action = &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
					},
				}
				addRouteToVirtualHost(route_, result, domainGroup)

				if len(location.ExtraDomainGroups) > 0 {
					for _, extraDG := range location.ExtraDomainGroups {
						dg := domainGroupMap[extraDG]
						if dg == nil {
							return nil, errors.Errorf("domain group %v not found", extraDG)
						}

						copyRoute := &route.Route{}
						copyMessage(copyRoute, route_)
						extraRoutes[dg.Name] = append(extraRoutes[dg.Name], copyRoute)
					}
				}
			}
		}
	}
	for _, vhost := range result {
		if extra, ok := extraRoutes[vhost.Name]; ok {
			vhost.Routes = append(vhost.Routes, extra...)
		}
	}
	return result, nil
}

func makeRoute(location *model.Location) (*route.Route, error) {
	var match *route.RouteMatch
	if location.Path != "" {
		match = &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: location.Path,
			},
		}
	} else if location.RePath != "" {
		match = &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{
					EngineType: &matcher.RegexMatcher_GoogleRe2{},
					Regex:      location.RePath,
				},
			},
		}
	} else {
		return nil, errors.Errorf("location path/re_path is empty")
	}
	result := &route.Route{
		Match: match,
	}
	return result, nil
}

func addRouteToVirtualHost(route_ *route.Route, vh map[string]*route.VirtualHost, dg *model.DomainGroup) {
	vhost := vh[dg.Name]
	if vhost == nil {
		vhost = &route.VirtualHost{
			Name:    dg.Name,
			Domains: dg.Domains,
			Routes:  nil,
		}
		vh[dg.Name] = vhost
	}
	vhost.Routes = append(vhost.Routes, route_)
}

func (p *Manager) createClusters(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {
	result := make([]envoytypes.Resource, 0)

	for _, static := range state.StaticUpstreams {
		clusterName := static.Name
		var endpoints []*endpoint.LocalityLbEndpoints
		for _, hostport := range static.Endpoints {
			host, port, err := net.SplitHostPort(hostport)
			if err != nil {
				return nil, errors.WithMessagef(err, "addr %v is invalid", hostport)
			}
			endpoints = append(endpoints, &endpoint.LocalityLbEndpoints{
				LbEndpoints: []*endpoint.LbEndpoint{
					{
						HostIdentifier: &endpoint.LbEndpoint_Endpoint{
							Endpoint: &endpoint.Endpoint{
								Address: &core.Address{
									Address: &core.Address_SocketAddress{
										SocketAddress: &core.SocketAddress{
											Address:  host,
											Protocol: core.SocketAddress_TCP,
											PortSpecifier: &core.SocketAddress_PortValue{
												PortValue: cast.ToUint32(port),
											},
										},
									},
								},
							},
						},
					},
				},
			})
		}

		clus := &cluster.Cluster{
			Name:           clusterName,
			ConnectTimeout: ptypes.DurationProto(time.Second),
			ClusterDiscoveryType: &cluster.Cluster_Type{
				Type: cluster.Cluster_STATIC,
			},
			DnsLookupFamily: cluster.Cluster_V4_ONLY,
			LbPolicy:        cluster.Cluster_ROUND_ROBIN,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints:   endpoints,
			},
		}
		result = append(result, clus)
	}

	for _, svcGroup := range state.ServiceGroups {
		for _, svc := range svcGroup.Services {
			clusterName := svc.Name
			clus := &cluster.Cluster{
				Name:           clusterName,
				ConnectTimeout: ptypes.DurationProto(time.Second),
				ClusterDiscoveryType: &cluster.Cluster_Type{
					Type: cluster.Cluster_EDS,
				},
				EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
					EdsConfig: &core.ConfigSource{
						ResourceApiVersion:    core.ApiVersion_V3,
						ConfigSourceSpecifier: &core.ConfigSource_Ads{},
					},
					ServiceName: clusterName,
				},
			}
			result = append(result, clus)
		}
	}

	return result, nil
}

func (p *Manager) createEndpoints(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {
	result := make([]envoytypes.Resource, 0)

	for _, svcGroup := range state.ServiceGroups {
		for _, svc := range svcGroup.Services {
			svcEndpoints, err := p.prov.DiscoverEndpoints(ctx, svc.Name)
			if err != nil {
				return nil, errors.AddStack(err)
			}
			cla := &endpoint.ClusterLoadAssignment{
				ClusterName: svc.Name,
				Endpoints:   nil,
			}
			for _, endp := range svcEndpoints {
				hostport := endp.Addr
				host, port, err := net.SplitHostPort(hostport)
				if err != nil {
					return nil, errors.AddStack(err)
				}

				cla.Endpoints = append(cla.Endpoints, &endpoint.LocalityLbEndpoints{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Protocol: core.SocketAddress_TCP,
												Address:  host,
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: cast.ToUint32(port),
												},
											},
										},
									},
								},
							},
						},
					},
				})
			}
			result = append(result, cla)
		}
	}

	return result, nil
}

func Run(provider provider.Provider, listenAddr string) (chan struct{}, error) {
	manager := NewManager(provider)
	err := manager.Watch()
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	err = manager.RunServer(ctx, listenAddr)
	if err != nil {
		return nil, err
	}

	exit := make(chan struct{})
	return exit, nil
}
