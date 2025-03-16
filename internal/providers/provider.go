package providers

import (
	"context"
	"fmt"
)

// internal/providers/provider.go
type Provider interface {
	UpdateRecord(ctx context.Context, domain, subdomain, ip string) error
}

type ProviderConfig map[string]string

type ProviderFactory interface {
	Create(config ProviderConfig) (Provider, error)
}

type ProviderManager struct {
	factories map[string]ProviderFactory
	providers map[string]Provider
}

func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		factories: make(map[string]ProviderFactory),
		providers: make(map[string]Provider),
	}
}

func (pm *ProviderManager) Register(name string, factory ProviderFactory) {
	pm.factories[name] = factory
}

func (pm *ProviderManager) CreateProvider(name string, config ProviderConfig) (Provider, error) {
	factory, ok := pm.factories[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return factory.Create(config)
}

func (pm *ProviderManager) Get(name string) Provider {
	return pm.providers[name]
}
