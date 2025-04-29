package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"CloudBalancer/internal/rate_limiter"

	"go.uber.org/zap"
)

type RateLimitHandler struct {
	rateLimiter rate_limiter.RateLimiter
	logger      *zap.Logger
}

func NewRateLimitHandler(rateLimiter rate_limiter.RateLimiter, logger *zap.Logger) *RateLimitHandler {
	return &RateLimitHandler{
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

type RateLimitRequest struct {
	Rate  float64 `json:"rate"`
	Burst int     `json:"burst"`
}

func (h *RateLimitHandler) HandleRateLimit(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Rate limit API request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		h.logger.Debug("Invalid rate limit API URL format")
		http.Error(w, "Invalid URL format. Use /admin/ratelimit/{clientID}", http.StatusBadRequest)
		return
	}
	clientID := parts[3]
	h.logger.Debug("Processing rate limit for client", zap.String("clientID", clientID))

	switch r.Method {
	case http.MethodGet:
		h.getRateLimit(w, clientID)
	case http.MethodPost:
		h.createRateLimit(w, r, clientID)
	case http.MethodPut:
		h.updateRateLimit(w, r, clientID)
	case http.MethodDelete:
		h.deleteRateLimit(w, clientID)
	default:
		h.logger.Debug("Unsupported method for rate limit API", zap.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *RateLimitHandler) getRateLimit(w http.ResponseWriter, clientID string) {
	h.logger.Debug("Getting rate limit for client", zap.String("clientID", clientID))

	limits := h.rateLimiter.GetClientLimits(clientID)
	response := RateLimitRequest{
		Rate:  limits.Rate,
		Burst: limits.Burst,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *RateLimitHandler) createRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	h.logger.Debug("Creating rate limit for client", zap.String("clientID", clientID))

	var limits RateLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		h.logger.Debug("Error decoding request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if limits.Rate <= 0 || limits.Burst <= 0 {
		h.logger.Debug("Invalid rate/burst values",
			zap.Float64("rate", limits.Rate),
			zap.Int("burst", limits.Burst),
		)
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	h.rateLimiter.SetClientLimits(clientID, limits.Rate, limits.Burst)
	h.logger.Info("Rate limit created for client",
		zap.String("clientID", clientID),
		zap.Float64("rate", limits.Rate),
		zap.Int("burst", limits.Burst),
	)

	w.WriteHeader(http.StatusCreated)
}

func (h *RateLimitHandler) updateRateLimit(w http.ResponseWriter, r *http.Request, clientID string) {
	h.logger.Debug("Updating rate limit for client", zap.String("clientID", clientID))

	var limits RateLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		h.logger.Debug("Error decoding request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if limits.Rate <= 0 || limits.Burst <= 0 {
		h.logger.Debug("Invalid rate/burst values",
			zap.Float64("rate", limits.Rate),
			zap.Int("burst", limits.Burst),
		)
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	h.rateLimiter.UpdateClientLimits(clientID, func(ul *rate_limiter.UserLimits) {
		ul.Rate = limits.Rate
		ul.Burst = limits.Burst
	})

	h.logger.Info("Rate limit updated for client",
		zap.String("clientID", clientID),
		zap.Float64("rate", limits.Rate),
		zap.Int("burst", limits.Burst),
	)

	w.WriteHeader(http.StatusOK)
}

func (h *RateLimitHandler) deleteRateLimit(w http.ResponseWriter, clientID string) {
	h.logger.Debug("Deleting rate limit for client", zap.String("clientID", clientID))

	h.rateLimiter.DeleteClientLimits(clientID)
	h.logger.Info("Rate limit deleted for client", zap.String("clientID", clientID))

	w.WriteHeader(http.StatusNoContent)
}
