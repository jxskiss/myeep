package provider

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/jxskiss/errors"
	"gopkg.in/yaml.v2"

	"github.com/jxskiss/myxdsdemo/pkg/model"
)

func NewFileProvider(rootDir string) Provider {
	return &fileProvider{
		rootDir: rootDir,
	}
}

type fileProvider struct {
	rootDir string
}

type domainGroups struct {
	DomainGroups []string `yaml:"domain_groups"`
}

func (p *fileProvider) ListDomainGroups(ctx context.Context) ([]*model.DomainGroup, error) {
	groupsFile := filepath.Join(p.rootDir, "domain_groups.yaml")
	data, err := ioutil.ReadFile(groupsFile)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	groups := &domainGroups{}
	err = yaml.Unmarshal(data, groups)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return p.readDomainGroupsFiles(ctx, groups.DomainGroups)
}

func (p *fileProvider) readDomainGroupsFiles(ctx context.Context, groups []string) ([]*model.DomainGroup, error) {
	result := make([]*model.DomainGroup, 0, len(groups))
	for _, groupName := range groups {
		groupFile := filepath.Join(p.rootDir, "domain_groups", groupName+".yaml")
		data, err := ioutil.ReadFile(groupFile)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		domainGroup := &model.DomainGroup{}
		err = yaml.Unmarshal(data, domainGroup)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		result = append(result, domainGroup)
	}
	return result, nil
}

func (p *fileProvider) ListServiceGroups(ctx context.Context) ([]*model.ServiceGroup, error) {
	result := make([]*model.ServiceGroup, 0)
	groupsDir := filepath.Join(p.rootDir, "service_groups")
	dirEntries, err := os.ReadDir(groupsDir)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	for _, f := range dirEntries {
		if f.IsDir() {
			continue
		}
		groupFile := filepath.Join(groupsDir, f.Name())
		data, err := ioutil.ReadFile(groupFile)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		serviceGroup := &model.ServiceGroup{}
		err = yaml.Unmarshal(data, serviceGroup)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		result = append(result, serviceGroup)
	}
	return result, nil
}

func (p *fileProvider) ListStaticUpstreams(ctx context.Context) ([]*model.StaticUpstream, error) {
	upstreamsFile := filepath.Join(p.rootDir, "static_upstreams.yaml")
	data, err := ioutil.ReadFile(upstreamsFile)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	upstreams := make([]*model.StaticUpstream, 0)
	err = yaml.Unmarshal(data, &upstreams)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return upstreams, nil
}

func (p *fileProvider) DiscoverEndpoints(ctx context.Context, serviceName string) ([]*model.Endpoint, error) {
	mockIp := func() string {
		return fmt.Sprintf("192.168.1.%d", rand.Intn(100)+1)
	}
	mockPort := func() int {
		return rand.Intn(8000) + 1000
	}

	n := rand.Intn(5) + 3 // mock at least 3 endpoints
	result := make([]*model.Endpoint, 0, n)
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("%s:%d", mockIp(), mockPort())
		result = append(result, &model.Endpoint{
			Addr:     addr,
			Cluster:  "default",
			Env:      "",
			Weight:   50,
			Metadata: nil,
		})
	}
	return result, nil
}

func (p *fileProvider) WatchConfig(ctx context.Context) <-chan struct{} {
	// TODO: watch file changes
	return nil
}

func (p *fileProvider) WatchEndpoints(ctx context.Context) <-chan struct{} {
	// TODO
	return nil
}
