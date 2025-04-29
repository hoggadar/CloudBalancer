package backend

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Backend struct {
	ID                string
	URL               *url.URL
	Proxy             *httputil.ReverseProxy
	isHealthy         bool
	activeConnections int64
	mtx               sync.RWMutex
}

func NewBackend(id string, url *url.URL, proxy *httputil.ReverseProxy) *Backend {
	return &Backend{
		ID:                id,
		URL:               url,
		Proxy:             proxy,
		isHealthy:         true,
		activeConnections: 0,
	}
}

func (b *Backend) IsHealthy() bool {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	return b.isHealthy
}

func (b *Backend) SetHealthy(healthy bool) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.isHealthy = healthy
}

func (b *Backend) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.activeConnections)
}

func (b *Backend) IncrementConnections() {
	atomic.AddInt64(&b.activeConnections, 1)
}

func (b *Backend) DecrementConnections() {
	atomic.AddInt64(&b.activeConnections, -1)
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.IncrementConnections()
	defer b.DecrementConnections()

	b.Proxy.ServeHTTP(w, r)
}

func ErrUnknownStrategy(name string) error {
	return fmt.Errorf("unknown balancing strategy: %s", name)
}
