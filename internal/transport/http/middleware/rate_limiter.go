package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"CloudBalancer/internal/rate_limiter"

	"go.uber.org/zap"
)

type RateLimiterMiddleware struct {
	rateLimiter rate_limiter.RateLimiter
	logger      *zap.Logger
}

func NewRateLimiterMiddleware(rateLimiter rate_limiter.RateLimiter, logger *zap.Logger) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

func (m *RateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/admin/") || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		clientID := getClientID(r)

		if !m.rateLimiter.Allow(clientID) {
			m.logger.Debug("Rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("path", r.URL.Path),
				zap.Float64("rate", m.rateLimiter.GetRate(clientID)),
				zap.Int("burst", m.rateLimiter.GetBurst(clientID)),
			)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Rate limit exceeded. Please slow down your requests.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientID(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "api:" + apiKey
	}

	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	return strings.Split(r.RemoteAddr, ":")[0]
}
