package router

import (
	"net/http"
	"time"

	"CloudBalancer/internal/load_balancer"
	"CloudBalancer/internal/rate_limiter"
	"CloudBalancer/internal/transport/http/handler"
	"CloudBalancer/internal/transport/http/middleware"

	"go.uber.org/zap"
)

type Router struct {
	mux          *http.ServeMux
	logger       *zap.Logger
	handler      *handler.Handler
	loadBalancer load_balancer.LoadBalancer
	rateLimiter  rate_limiter.RateLimiter
}

func NewRouter(logger *zap.Logger, lb load_balancer.LoadBalancer, rl rate_limiter.RateLimiter) *Router {
	return &Router{
		mux:          http.NewServeMux(),
		logger:       logger,
		loadBalancer: lb,
		rateLimiter:  rl,
		handler:      handler.NewHandler(lb, rl, logger),
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	path := req.URL.Path
	raw := req.URL.RawQuery

	captureWriter := &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	r.mux.ServeHTTP(captureWriter, req)

	latency := time.Since(start)
	clientIP := req.RemoteAddr
	method := req.Method
	statusCode := captureWriter.statusCode

	if raw != "" {
		path = path + "?" + raw
	}

	r.logger.Info("Request processed",
		zap.String("path", path),
		zap.String("client_ip", clientIP),
		zap.String("method", method),
		zap.Int("status_code", statusCode),
		zap.Duration("latency", latency),
	)
}

func (r *Router) SetupRoutes() {
	rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(r.rateLimiter, r.logger)

	r.mux.HandleFunc("/health", r.handler.HealthCheck)
	r.mux.Handle("/", rateLimiterMiddleware.Middleware(http.HandlerFunc(r.handler.LoadBalancer)))
	r.mux.HandleFunc("/admin/stats", r.handler.AdminGetStats)
	r.mux.HandleFunc("/admin/strategy", r.handler.AdminChangeStrategy)
	r.mux.HandleFunc("/admin/ratelimit/", r.handler.RateLimitHandler)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
