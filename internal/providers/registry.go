package providers

import (
	"fmt"
	"sort"
	"sync"
)

// Registry provides thread-safe storage and lookup for provider adapters.
// It maintains a map of provider names to their implementations and ensures
// concurrent access is safe through mutex protection.
//
// The registry is used during application startup to register all available
// providers, and during request handling to select the appropriate provider
// based on configuration.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new empty provider registry.
// The registry is initialized with an empty map and is ready to accept
// provider registrations.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry with the given name.
// The name should match the provider's Name() method return value.
//
// Returns an error if a provider with the same name is already registered.
// This prevents accidental overwrites and ensures provider uniqueness.
//
// Thread-safe: Uses write lock to protect concurrent registration attempts.
func (r *Registry) Register(name string, provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Get retrieves a provider by name from the registry.
// Returns the provider implementation and nil error if found.
// Returns nil and an error if the provider is not registered.
//
// Thread-safe: Uses read lock to allow concurrent lookups without blocking.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// List returns a sorted slice of all registered provider names.
// The names are sorted alphabetically for consistent ordering.
//
// This method is useful for:
// - Displaying available providers to users
// - Logging registered providers at startup
// - Validating configuration against available providers
//
// Thread-safe: Uses read lock and creates a copy of the names to prevent
// concurrent modification issues.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
