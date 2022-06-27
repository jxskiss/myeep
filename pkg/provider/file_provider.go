package provider

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"

	"github.com/jxskiss/errors"
	"gopkg.in/yaml.v3"

	"github.com/jxskiss/myeep/pkg/api"
)

func NewFileProvider(rootDir string) Provider {
	return &fileProvider{
		rootDir: rootDir,
	}
}

type fileProvider struct {
	rootDir string
}

type domainGroupIndex struct {
	DomainGroups []string `yaml:"domain_groups"`
}

type serviceIndex struct {
	Services []string `yaml:"services"`
}

func (p *fileProvider) ListDomainGroups(ctx context.Context) ([]*api.DomainGroup, error) {
	indexFile := filepath.Join(p.rootDir, "domain_group_index.yaml")
	data, err := ioutil.ReadFile(indexFile)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	groupIndex := &domainGroupIndex{}
	err = yaml.Unmarshal(data, groupIndex)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return p.readDomainGroupsFiles(ctx, groupIndex.DomainGroups)
}

func (p *fileProvider) readDomainGroupsFiles(ctx context.Context, groups []string) ([]*api.DomainGroup, error) {
	result := make([]*api.DomainGroup, 0, len(groups))
	for _, groupName := range groups {
		groupFile := filepath.Join(p.rootDir, "domain_groups", groupName+".yaml")
		data, err := ioutil.ReadFile(groupFile)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		domainGroup := &api.DomainGroup{}
		err = yaml.Unmarshal(data, domainGroup)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		result = append(result, domainGroup)
	}
	return result, nil
}

func (p *fileProvider) ListServices(ctx context.Context) ([]*api.Service, error) {
	indexFile := filepath.Join(p.rootDir, "service_index.yaml")
	data, err := ioutil.ReadFile(indexFile)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	serviceIdx := &serviceIndex{}
	err = yaml.Unmarshal(data, serviceIdx)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return p.readServiceFiles(ctx, serviceIdx.Services)
}

func (p *fileProvider) readServiceFiles(ctx context.Context, services []string) ([]*api.Service, error) {
	result := make([]*api.Service, 0)
	for _, srvName := range services {
		srvFile := filepath.Join(p.rootDir, "services", srvName+".yaml")
		data, err := ioutil.ReadFile(srvFile)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		service := &api.Service{}
		err = yaml.Unmarshal(data, service)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		result = append(result, service)
	}
	return result, nil
}

func (p *fileProvider) WatchConfig(ctx context.Context) <-chan struct{} {
	// TODO
	return nil
}

func (p *fileProvider) DiscoverEndpoints(ctx context.Context, cluster string) ([]*api.Endpoint, error) {
	mockIp := func() string {
		return fmt.Sprintf("192.168.1.%d", rand.Intn(100)+1)
	}
	mockPort := func() int {
		return rand.Intn(8000) + 1000
	}

	n := rand.Intn(5) + 3 // mock at least 3 endpoints
	result := make([]*api.Endpoint, 0, n)
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("%s:%d", mockIp(), mockPort())
		result = append(result, &api.Endpoint{
			Addr:     addr,
			Cluster:  "default",
			Env:      "",
			Weight:   50,
			Metadata: nil,
		})
	}
	return result, nil
}

func (p *fileProvider) WatchEndpoints(ctx context.Context) <-chan struct{} {
	// TODO
	return nil
}
