package myxds

import (
	"context"
	"fmt"
	"sync"

	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/jxskiss/gopkg/exp/zlog"
)

type Callbacks struct {
	mu       sync.Mutex
	fetches  int
	requests int
}

func (cb *Callbacks) Report() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	zlog.Infof("report callbacks, fetches=%v requests=%v", cb.fetches, cb.requests)
}

func (cb *Callbacks) OnStreamOpen(_ context.Context, id int64, typ string) error {
	zlog.Infof("OnStreamOpen %d open for %s", id, typ)
	return nil
}

func (cb *Callbacks) OnStreamClosed(id int64) {
	zlog.Infof("OnStreamClosed %d closed", id)
}

func (cb *Callbacks) OnStreamRequest(id int64, r *discoverygrpc.DiscoveryRequest) error {
	nodeInfo := fmt.Sprintf("%v/%v", r.Node.Cluster, r.Node.Id)
	zlog.Infof("OnStreamRequest %v, %v", nodeInfo, r.TypeUrl)
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.requests++
	return nil
}

func (cb *Callbacks) OnStreamResponse(context.Context, int64, *discoverygrpc.DiscoveryRequest, *discoverygrpc.DiscoveryResponse) {
	zlog.Infof("OnStreamResponse...")
	cb.Report()
}

func (cb *Callbacks) OnFetchRequest(ctx context.Context, req *discoverygrpc.DiscoveryRequest) error {
	nodeInfo := fmt.Sprintf("%v/%v", req.Node.Cluster, req.Node.Id)
	zlog.Infof("OnFetchRequest %v, %v", nodeInfo, req.TypeUrl)
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.fetches++
	return nil
}

func (cb *Callbacks) OnFetchResponse(*discoverygrpc.DiscoveryRequest, *discoverygrpc.DiscoveryResponse) {
	panic("implement me")
}

func (cb *Callbacks) OnDeltaStreamOpen(ctx context.Context, i int64, s string) error {
	panic("implement me")
}

func (cb *Callbacks) OnDeltaStreamClosed(i int64) {
	panic("implement me")
}

func (cb *Callbacks) OnStreamDeltaRequest(i int64, request *discoverygrpc.DeltaDiscoveryRequest) error {
	panic("implement me")
}

func (cb *Callbacks) OnStreamDeltaResponse(i int64, request *discoverygrpc.DeltaDiscoveryRequest, response *discoverygrpc.DeltaDiscoveryResponse) {
	panic("implement me")
}
