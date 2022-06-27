package myxds

import (
	"context"
	"fmt"
	"net"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
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
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/easy"
	"github.com/jxskiss/gopkg/v2/json"
	"github.com/jxskiss/gopkg/v2/set"
	"github.com/jxskiss/gopkg/v2/zlog"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/jxskiss/myeep/pkg/api"
	"github.com/jxskiss/myeep/pkg/provider"
)

const grpcMaxConcurrentStreams = 100000

type ConfigState struct {
	DomainGroups []*api.DomainGroup
	Services     []*api.Service
}

type clusterNameHash struct{}

func (_ clusterNameHash) ID(node *core.Node) string {
	if node == nil {
		return ""
	}
	return node.Cluster
}

type Server struct {
	prov      provider.Provider
	cache     envoycache.SnapshotCache
	callbacks *Callbacks
}

func NewManager(prov provider.Provider) *Server {
	cache := envoycache.NewSnapshotCache(true, clusterNameHash{}, zlog.S())
	return &Server{
		prov:      prov,
		cache:     cache,
		callbacks: &Callbacks{},
	}
}

func (p *Server) Watch() error {
	ctx := context.Background()
	state, err := p.readConfigState(ctx)
	if err != nil {
		return err
	}

	easy.PanicOnError(json.Dump("./conf/generated/state_dump.json", state, "", "  "))

	snap, err := p.createSnapshot(ctx, state)
	if err != nil {
		return err
	}

	clusterName := "infra.l7lb.myxds_demo_envoy" // FIXME
	err = p.cache.SetSnapshot(ctx, clusterName, snap)
	if err != nil {
		return errors.WithMessage(err, "failed set snapshot")
	}

	// TODO: watch implementation

	return nil
}

func (p *Server) RunServer(ctx context.Context, addr string) error {

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

func (p *Server) readConfigState(ctx context.Context) (*ConfigState, error) {
	domainGroups, err := p.prov.ListDomainGroups(ctx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	services, err := p.prov.ListServices(ctx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return &ConfigState{
		DomainGroups: domainGroups,
		Services:     services,
	}, nil
}

func (p *Server) createSnapshot(ctx context.Context, state *ConfigState) (envoycache.ResourceSnapshot, error) {
	listeners, err := p.createListeners(ctx, state)
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
	secrets, err := p.createSecrets(ctx, state)
	if err != nil {
		return nil, err
	}
	endpoints, err := p.createEndpoints(ctx, state)
	if err != nil {
		return nil, err
	}

	version := fmt.Sprint(time.Now().UnixNano())
	snap, err := envoycache.NewSnapshot(version, map[resource.Type][]envoytypes.Resource{
		resource.EndpointType: endpoints,
		resource.ClusterType:  clusters,
		resource.RouteType:    virtualHosts,
		resource.ListenerType: listeners,
		resource.SecretType:   secrets,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "failed create snapshot")
	}
	if err = snap.Consistent(); err != nil {
		return nil, errors.WithMessage(err, "snapshot is not consistent")
	}
	return snap, nil
}

func (p *Server) createListeners(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {
	result := make([]envoytypes.Resource, 0)
	listenerMap := make(map[string]*listener.Listener)

	portSet := set.New[uint]()
	for _, svc := range state.Services {
		for _, svcRoute := range svc.Routes {
			for _, domainGrp := range svcRoute.DomainGroups {
				ports := domainGrp.Ports
				if len(ports) == 0 {
					ports = []uint{80}
				}
				portSet.Add(ports...)
			}
		}
	}

	portList := easy.Sort(portSet.Slice())
	for _, port := range portList {

		// TODO: envoy cluster name
		listenerName := getListenerName(port)
		if listenerMap[listenerName] != nil {
			continue
		}

		httpFilterPbst, _ := anypb.New(&routerv3.Router{})
		manager := &hcm.HttpConnectionManager{
			CodecType:  hcm.HttpConnectionManager_AUTO,
			StatPrefix: "ingress_http",
			RouteSpecifier: &hcm.HttpConnectionManager_Rds{
				Rds: &hcm.Rds{
					RouteConfigName: listenerName,
					ConfigSource: &core.ConfigSource{
						ResourceApiVersion: resource.DefaultAPIVersion,
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
					},
				},
			},
			HttpFilters: []*hcm.HttpFilter{{
				Name: wellknown.Router,
				ConfigType: &hcm.HttpFilter_TypedConfig{
					TypedConfig: httpFilterPbst,
				},
			}},
		}
		pbst, err := anypb.New(manager)
		if err != nil {
			return nil, err
		}

		listener_ := &listener.Listener{
			Name: listenerName,
			Address: &core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Protocol: core.SocketAddress_TCP,
						Address:  "0.0.0.0",
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: uint32(port),
						},
					},
				},
			},
			FilterChains: []*listener.FilterChain{
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
			},
		}
		result = append(result, listener_)
	}
	return result, nil
}

func (p *Server) createVirtualHosts(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {

	result := make([]envoytypes.Resource, 0)
	vhostMap := make(map[string]*route.VirtualHost)

	domainGroupMap := make(map[string]*api.DomainGroup)
	for _, group := range state.DomainGroups {
		domainGroupMap[group.Name] = group
	}

	portDomainGroupMap := make(map[uint][]string)

	for _, service := range state.Services {

		// TODO: directives

		clusterName := service.Cluster

		for _, svcRoute := range service.Routes {

			routes_ := make([]*route.Route, 0, len(svcRoute.Locations))
			for _, loc := range svcRoute.Locations {
				route_, err := makeRoute(loc)
				if err != nil {
					return nil, errors.WithMessage(err, "make route")
				}
				route_.Action = &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
					},
				}
				routes_ = append(routes_, route_)
			}

			for _, routeDomainGrpAndPorts := range svcRoute.DomainGroups {
				domainGrpName := routeDomainGrpAndPorts.Name
				domainGroup := domainGroupMap[domainGrpName]
				if domainGroup == nil {
					return nil, errors.Errorf("domain group %v not exists", domainGrpName)
				}

				ports := routeDomainGrpAndPorts.Ports
				if len(ports) == 0 {
					ports = []uint{80}
				}
				for _, port := range ports {

					if !easy.InStrings(portDomainGroupMap[port], domainGrpName) {
						portDomainGroupMap[port] = append(portDomainGroupMap[port], domainGrpName)
					}

					vhostName := getVirtualHostName(port, domainGrpName)
					vhost := vhostMap[vhostName]
					if vhost == nil {
						vhost = &route.VirtualHost{
							Name: vhostName,
						}
						vhostMap[vhostName] = vhost
					}
					for _, dn := range domainGroup.Domains {
						if !easy.InStrings(vhost.Domains, dn) {
							vhost.Domains = append(vhost.Domains, dn)
						}
						dnWithPort := fmt.Sprintf("%s:%d", dn, port)
						if !easy.InStrings(vhost.Domains, dnWithPort) {
							vhost.Domains = append(vhost.Domains, dnWithPort)
						}
					}
					vhost.Routes = append(vhost.Routes, routes_...)
				}
			}
		}
	}

	rcMap := make(map[string]*route.RouteConfiguration)
	for port, domainGroupNames := range portDomainGroupMap {
		for _, domainGrpName := range domainGroupNames {
			listenerName := getListenerName(port)
			vhostName := getVirtualHostName(port, domainGrpName)
			vhost := vhostMap[vhostName]
			if vhost == nil {
				return nil, errors.Errorf("got unexpected nil virtualHost %s", vhostName)
			}
			rc := rcMap[listenerName]
			if rc == nil {
				rc = &route.RouteConfiguration{
					Name: listenerName,
					ValidateClusters: &wrappers.BoolValue{
						Value: false,
					},
					IgnorePortInHostMatching: true,
				}
				rcMap[listenerName] = rc
				result = append(result, rc)
			}
			rc.VirtualHosts = append(rc.VirtualHosts, vhost)
		}
	}

	zlog.TRACE("virtualHosts result: %v", result)

	return result, nil
}

func makeRoute(location *api.Location) (*route.Route, error) {
	var match *route.RouteMatch
	if location.Path != "" {
		match = &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: location.Path,
			},
		}
	} else if location.RegexPath != "" {
		match = &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_SafeRegex{
				SafeRegex: &matcher.RegexMatcher{
					EngineType: &matcher.RegexMatcher_GoogleRe2{},
					Regex:      location.RegexPath,
				},
			},
		}
	} else {
		return nil, errors.Errorf("location path/regex_path is empty")
	}
	result := &route.Route{
		Match: match,
	}
	return result, nil
}

func (p *Server) createClusters(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {

	result := make([]envoytypes.Resource, 0)
	clusterSet := set.New[string]()

	addXdsCluster := func(serviceName string) {
		if clusterSet.Contains(serviceName) {
			return
		}
		clus := p.makeCluster(serviceName)
		result = append(result, clus)
		clusterSet.Add(serviceName)
	}

	for _, service := range state.Services {
		clusterName := service.Cluster
		addXdsCluster(clusterName)
		for _, svcRoute := range service.Routes {
			for _, loc := range svcRoute.Locations {
				for _, spl := range loc.Splitting {
					addXdsCluster(spl.DestCluster)
				}
			}
		}
	}

	zlog.TRACE("clusters result: %v", result)

	return result, nil
}

func (p *Server) makeCluster(serviceName string) *cluster.Cluster {
	zlog.TRACE("makeCluster serviceName = %v", serviceName)
	clus := &cluster.Cluster{
		Name:           serviceName,
		ConnectTimeout: durationpb.New(time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{
			Type: cluster.Cluster_EDS,
		},
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			ServiceName: serviceName,
			EdsConfig: &core.ConfigSource{
				ResourceApiVersion: resource.DefaultAPIVersion,
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		},
	}
	return clus
}

func (p *Server) createEndpoints(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {

	result := make([]envoytypes.Resource, 0)
	clusterSet := set.New[string]()

	addEndpoints := func(serviceName string) error {
		if clusterSet.Contains(serviceName) {
			return nil
		}
		endpoints, err := p.prov.DiscoverEndpoints(ctx, serviceName)
		if err != nil {
			return errors.AddStack(err)
		}
		cla := &endpoint.ClusterLoadAssignment{
			ClusterName: serviceName,
			Endpoints:   nil,
		}
		for _, endp := range endpoints {
			hostport := endp.Addr
			host, port, err := net.SplitHostPort(hostport)
			if err != nil {
				return errors.AddStack(err)
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
		clusterSet.Add(serviceName)
		return nil
	}

	for _, service := range state.Services {
		clusterName := service.Cluster
		if err := addEndpoints(clusterName); err != nil {
			return nil, err
		}
		for _, svcRoute := range service.Routes {
			for _, loc := range svcRoute.Locations {
				for _, spl := range loc.Splitting {
					if err := addEndpoints(spl.DestCluster); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	zlog.TRACE("endpoints result: %v", result)

	return result, nil
}

/*
func generateSnapshot(notification *common.Notification) cache.Snapshot {
	var resources []types.Resource

	for _, cert := range notification.Certificates {
		secret := &envoy_extensions_transport_sockets_tls_v3.Secret{
			Name: cert.Domain,
			Type: &envoy_extensions_transport_sockets_tls_v3.Secret_TlsCertificate{
				TlsCertificate: &envoy_extensions_transport_sockets_tls_v3.TlsCertificate{
					CertificateChain: &envoy_config_core_v3.DataSource{
						Specifier: &envoy_config_core_v3.DataSource_InlineBytes{
							InlineBytes: cert.Certificate,
						},
					},
					PrivateKey: &envoy_config_core_v3.DataSource{
						Specifier: &envoy_config_core_v3.DataSource_InlineBytes{
							InlineBytes: cert.PrivateKey,
						},
					},
				},
			},
		}
		resources = append(resources, secret)
	}

	return cache.NewSnapshot(fmt.Sprintf("%d", time.Now().Unix()), nil, nil, nil, nil, nil, resources)
}
*/

func (p *Server) createSecrets(ctx context.Context, state *ConfigState) ([]envoytypes.Resource, error) {

	result := make([]envoytypes.Resource, 0)
	_ = result

	// TODO
	return nil, nil
}

func Run(provider provider.Provider, listenAddr string) (chan struct{}, error) {
	manager := NewManager(provider)
	err := manager.Watch()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	err = manager.RunServer(ctx, listenAddr)
	if err != nil {
		return nil, err
	}

	exit := make(chan struct{})
	return exit, nil
}
