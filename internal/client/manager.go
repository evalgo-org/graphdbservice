// Package client manages HTTP client creation and configuration.
package client

import (
	"net/http"
	"sync"

	"eve.evalgo.org/db"
	"graphdbservice/internal/helpers"
)

// Manager handles HTTP client creation and caching
type Manager struct {
	identityFile string
	debug        bool
	cache        map[string]*http.Client
	mu            sync.RWMutex
}

// NewManager creates a new client manager
func NewManager(identityFile string, debug bool) *Manager {
	return &Manager{
		identityFile: identityFile,
		debug:        debug,
		cache:        make(map[string]*http.Client),
	}
}

// GetClient returns an HTTP client for the given GraphDB server.
// If Ziti identity is configured, it returns a Ziti-enabled client.
// Otherwise, it returns the default HTTP client.
// Clients are cached to avoid recreating them.
func (m *Manager) GetClient(serverURL string) (*http.Client, error) {
	if m.identityFile == "" {
		// No Ziti identity, use default client
		client := &http.Client{}
		if m.debug {
			client = helpers.EnableHTTPDebugLogging(client)
		}
		return client, nil
	}

	// With Ziti identity, check cache first
	m.mu.RLock()
	if cachedClient, exists := m.cache[serverURL]; exists {
		m.mu.RUnlock()
		return cachedClient, nil
	}
	m.mu.RUnlock()

	// Create new Ziti client
	hostname, err := helpers.URL2ServiceRobust(serverURL)
	if err != nil {
		return nil, err
	}

	client, err := db.GraphDBZitiClient(m.identityFile, hostname)
	if err != nil {
		return nil, err
	}

	if m.debug {
		client = helpers.EnableHTTPDebugLogging(client)
	}

	// Cache the client
	m.mu.Lock()
	m.cache[serverURL] = client
	m.mu.Unlock()

	return client, nil
}

// ClearCache clears the cached clients
func (m *Manager) ClearCache() {
	m.mu.Lock()
	m.cache = make(map[string]*http.Client)
	m.mu.Unlock()
}
