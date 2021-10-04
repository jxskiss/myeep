package provider

import (
	"context"

	"github.com/jxskiss/myxdsdemo/pkg/model"
)

type Provider interface {
	ListDomainGroups(ctx context.Context) ([]*model.DomainGroup, error)
	ListServiceGroups(ctx context.Context) ([]*model.ServiceGroup, error)
	ListStaticUpstreams(ctx context.Context) ([]*model.StaticUpstream, error)
	DiscoverEndpoints(ctx context.Context, serviceName string) ([]*model.Endpoint, error)

	WatchConfig(ctx context.Context) <-chan struct{}
	WatchEndpoints(ctx context.Context) <-chan struct{}
}
