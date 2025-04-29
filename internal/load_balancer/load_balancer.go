package load_balancer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"CloudBalancer/config"
	"CloudBalancer/internal/load_balancer/algorithm"
	"CloudBalancer/internal/load_balancer/backend"

	"go.uber.org/zap"
)

type LoadBalancer interface {
	GetNextBackend() (*backend.Backend, error)
	HealthCheck(ctx context.Context)
	GetBackends() []*backend.Backend
	GetStrategy() algorithm.Strategy
	SetStrategy(strategy algorithm.Strategy)
}

type loadBalancer struct {
	backends    []*backend.Backend
	strategy    algorithm.Strategy
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *config.Config
	healthCheck *http.Client
}

func NewLoadBalancer(config *config.Config, logger *zap.Logger) (LoadBalancer, error) {
	strategy, err := algorithm.GetStrategy(config.LoadBalancer.Method)
	if err != nil {
		return nil, fmt.Errorf("failed to create balancing strategy: %w", err)
	}

	lb := &loadBalancer{
		strategy: strategy,
		logger:   logger,
		config:   config,
		healthCheck: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   3 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}

	for _, backendConfig := range config.Backends {
		if !backendConfig.Enabled {
			continue
		}

		backendURL, err := url.Parse(fmt.Sprintf("http://%s:%d", backendConfig.Host, backendConfig.Port))
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL: %w", err)
		}

		transport := createTransport(backendConfig.ConnectTimeout, backendConfig.ReadTimeout)

		proxy := httputil.NewSingleHostReverseProxy(backendURL)
		proxy.Transport = transport

		setupDirector(proxy, backendConfig.ID)

		setupErrorHandler(proxy, backendConfig.ID, logger)

		b := backend.NewBackend(
			backendConfig.ID,
			backendURL,
			proxy,
		)

		lb.backends = append(lb.backends, b)
	}

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("no enabled backends configured")
	}

	go lb.startHealthCheck()

	logger.Info("Load balancer initialized",
		zap.String("strategy", strategy.Name()),
		zap.Int("backends", len(lb.backends)),
	)

	return lb, nil
}

func createTransport(connectTimeout, readTimeout time.Duration) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: readTimeout,
	}
}

func setupDirector(proxy *httputil.ReverseProxy, backendID string) {
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "http")

		req.Header.Set("X-Load-Balancer", "CloudBalancer")
		req.Header.Set("X-Backend", backendID)
	}
}

func setupErrorHandler(proxy *httputil.ReverseProxy, backendID string, logger *zap.Logger) {
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error",
			zap.String("backend", backendID),
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error": "Backend server error"}`))
	}
}

func (lb *loadBalancer) GetNextBackend() (*backend.Backend, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	b, err := lb.strategy.NextBackend(lb.backends)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (lb *loadBalancer) GetBackends() []*backend.Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	backends := make([]*backend.Backend, len(lb.backends))
	copy(backends, lb.backends)

	return backends
}

func (lb *loadBalancer) GetStrategy() algorithm.Strategy {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.strategy
}

func (lb *loadBalancer) SetStrategy(strategy algorithm.Strategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
	lb.logger.Info("Load balancing strategy changed", zap.String("strategy", strategy.Name()))
}

func (lb *loadBalancer) startHealthCheck() {
	ticker := time.NewTicker(lb.config.LoadBalancer.HealthCheckInterval)
	defer ticker.Stop()

	lb.HealthCheck(context.Background())

	for range ticker.C {
		lb.HealthCheck(context.Background())
	}
}

func (lb *loadBalancer) HealthCheck(ctx context.Context) {
	for _, b := range lb.backends {
		go lb.checkBackendHealth(ctx, b)
	}
}

func (lb *loadBalancer) checkBackendHealth(ctx context.Context, b *backend.Backend) {
	healthURL := fmt.Sprintf("%s/health", b.URL.String())
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		lb.logger.Error("Failed to create health check request",
			zap.String("backend", b.ID),
			zap.Error(err),
		)
		return
	}

	resp, err := lb.healthCheck.Do(req)
	if err != nil {
		lb.logger.Warn("Health check connection failed",
			zap.String("backend", b.ID),
			zap.Error(err),
		)
		wasHealthy := b.IsHealthy()
		b.SetHealthy(false)

		if wasHealthy {
			lb.logger.Warn("Backend became unhealthy due to connection error",
				zap.String("backend", b.ID),
			)
		}
		return
	}
	defer resp.Body.Close()

	isHealthy := resp.StatusCode == http.StatusOK
	wasHealthy := b.IsHealthy()
	b.SetHealthy(isHealthy)

	if wasHealthy != isHealthy {
		if isHealthy {
			lb.logger.Info("Backend became healthy",
				zap.String("backend", b.ID),
			)
		} else {
			lb.logger.Warn("Backend became unhealthy",
				zap.String("backend", b.ID),
				zap.Int("status_code", resp.StatusCode),
			)
		}
	}
}
