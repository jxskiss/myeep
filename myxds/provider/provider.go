package provider

import (
	"context"

	"github.com/jxskiss/myxdsdemo/pkg/api"
)

type Provider interface {
	ListDomainGroups(ctx context.Context) ([]*api.DomainGroup, error)
	ListServices(ctx context.Context) ([]*api.Service, error)
	WatchConfig(ctx context.Context) <-chan struct{}

	DiscoverEndpoints(ctx context.Context, cluster string) ([]*api.Endpoint, error)
	WatchEndpoints(ctx context.Context) <-chan struct{}
}
