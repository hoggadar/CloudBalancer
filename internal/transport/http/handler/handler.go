package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"CloudBalancer/internal/load_balancer"
	"CloudBalancer/internal/load_balancer/algorithm"
	"CloudBalancer/internal/rate_limiter"

	"go.uber.org/zap"
)

type Handler struct {
	loadBalancer load_balancer.LoadBalancer
	rateLimiter  rate_limiter.RateLimiter
	logger       *zap.Logger
	rateHandler  *RateLimitHandler
}

func NewHandler(lb load_balancer.LoadBalancer, rl rate_limiter.RateLimiter, logger *zap.Logger) *Handler {
	rateHandler := NewRateLimitHandler(rl, logger)

	return &Handler{
		loadBalancer: lb,
		rateLimiter:  rl,
		logger:       logger,
		rateHandler:  rateHandler,
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (h *Handler) LoadBalancer(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	backend, err := h.loadBalancer.GetNextBackend()
	if err != nil {
		h.logger.Error("Failed to get next backend",
			zap.String("path", r.URL.Path),
			zap.String("client_ip", r.RemoteAddr),
			zap.Error(err),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No healthy backends available",
		})
		return
	}

	h.logger.Info("Request forwarded to backend",
		zap.String("path", r.URL.Path),
		zap.String("client_ip", r.RemoteAddr),
		zap.String("backend_id", backend.ID),
		zap.String("backend_url", backend.URL.String()),
		zap.Int64("active_connections", backend.ActiveConnections()),
	)

	backend.ServeHTTP(w, r)

	elapsed := time.Since(startTime)
	h.logger.Info("Backend response completed",
		zap.String("path", r.URL.Path),
		zap.String("client_ip", r.RemoteAddr),
		zap.String("backend_id", backend.ID),
		zap.Duration("response_time", elapsed),
	)
}

type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newCaptureResponseWriter(w http.ResponseWriter) *captureResponseWriter {
	return &captureResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (crw *captureResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

func (h *Handler) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	backends := h.loadBalancer.GetBackends()

	type backendStat struct {
		ID                string `json:"id"`
		URL               string `json:"url"`
		Healthy           bool   `json:"healthy"`
		ActiveConnections int64  `json:"active_connections"`
	}

	stats := make([]backendStat, 0, len(backends))
	for _, backend := range backends {
		stats = append(stats, backendStat{
			ID:                backend.ID,
			URL:               backend.URL.String(),
			Healthy:           backend.IsHealthy(),
			ActiveConnections: backend.ActiveConnections(),
		})
	}

	response := map[string]interface{}{
		"strategy": h.loadBalancer.GetStrategy().Name(),
		"backends": stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) AdminChangeStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Strategy string `json:"strategy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	strategy, err := algorithm.GetStrategy(request.Strategy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.loadBalancer.SetStrategy(strategy)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Strategy changed successfully",
		"strategy": strategy.Name(),
	})
}

func (h *Handler) RateLimitHandler(w http.ResponseWriter, r *http.Request) {
	h.rateHandler.HandleRateLimit(w, r)
}
