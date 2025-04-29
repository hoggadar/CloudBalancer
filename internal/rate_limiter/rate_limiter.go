package rate_limiter

import (
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type UserLimits struct {
	Rate  float64
	Burst int
}

type RateLimiter interface {
	Allow(clientID string) bool
	Wait(clientID string) time.Duration
	Reserve(clientID string) time.Duration
	GetTokens(clientID string) float64
	GetBurst(clientID string) int
	GetRate(clientID string) float64
	SetClientLimits(clientID string, rate float64, burst int)
	GetClientLimits(clientID string) *UserLimits
	DeleteClientLimits(clientID string)
	UpdateClientLimits(clientID string, updateFn func(*UserLimits))
}

type TokenBucket struct {
	defaultRate  float64
	defaultBurst int
	limiters     sync.Map
	clientLimits sync.Map
	logger       *zap.Logger
	mtx          sync.RWMutex
}

func NewTokenBucket(defaultRate float64, defaultBurst int, logger *zap.Logger) *TokenBucket {
	logger.Info("Initializing token bucket rate limiter",
		zap.Float64("defaultRate", defaultRate),
		zap.Int("defaultBurst", defaultBurst),
	)

	return &TokenBucket{
		defaultRate:  defaultRate,
		defaultBurst: defaultBurst,
		logger:       logger,
	}
}

func (tb *TokenBucket) Allow(clientID string) bool {
	limiter := tb.getLimiter(clientID)
	allowed := limiter.Allow()

	if !allowed {
		tb.logger.Debug("Rate limit exceeded",
			zap.String("clientID", clientID),
			zap.Float64("rate", tb.GetRate(clientID)),
			zap.Int("burst", tb.GetBurst(clientID)),
		)
	}

	return allowed
}

func (tb *TokenBucket) SetClientLimits(clientID string, myrate float64, burst int) {
	tb.mtx.Lock()
	defer tb.mtx.Unlock()

	tb.clientLimits.Store(clientID, &UserLimits{
		Rate:  myrate,
		Burst: burst,
	})

	limiter := rate.NewLimiter(rate.Limit(myrate), burst)
	tb.limiters.Store(clientID, limiter)

	tb.logger.Info("Client rate limits set",
		zap.String("clientID", clientID),
		zap.Float64("rate", myrate),
		zap.Int("burst", burst),
	)
}

func (tb *TokenBucket) GetClientLimits(clientID string) *UserLimits {
	if limits, ok := tb.clientLimits.Load(clientID); ok {
		return limits.(*UserLimits)
	}
	return &UserLimits{
		Rate:  tb.defaultRate,
		Burst: tb.defaultBurst,
	}
}

func (tb *TokenBucket) DeleteClientLimits(clientID string) {
	tb.mtx.Lock()
	defer tb.mtx.Unlock()

	tb.clientLimits.Delete(clientID)
	tb.limiters.Delete(clientID)

	tb.logger.Info("Client rate limits deleted", zap.String("clientID", clientID))
}

func (tb *TokenBucket) UpdateClientLimits(clientID string, updateFn func(*UserLimits)) {
	tb.mtx.Lock()
	defer tb.mtx.Unlock()

	limits := tb.GetClientLimits(clientID)
	updateFn(limits)

	tb.clientLimits.Store(clientID, limits)

	limiter := rate.NewLimiter(rate.Limit(limits.Rate), limits.Burst)
	tb.limiters.Store(clientID, limiter)

	tb.logger.Info("Client rate limits updated",
		zap.String("clientID", clientID),
		zap.Float64("rate", limits.Rate),
		zap.Int("burst", limits.Burst),
	)
}

func (tb *TokenBucket) Wait(clientID string) time.Duration {
	limiter := tb.getLimiter(clientID)
	now := time.Now()
	limiter.Wait(nil)
	return time.Since(now)
}

func (tb *TokenBucket) Reserve(clientID string) time.Duration {
	limiter := tb.getLimiter(clientID)
	return limiter.Reserve().Delay()
}

func (tb *TokenBucket) getLimiter(clientID string) *rate.Limiter {
	if limiter, ok := tb.limiters.Load(clientID); ok {
		return limiter.(*rate.Limiter)
	}

	limits := tb.GetClientLimits(clientID)

	limiter := rate.NewLimiter(rate.Limit(limits.Rate), limits.Burst)
	tb.limiters.Store(clientID, limiter)

	tb.logger.Debug("Created new rate limiter for client",
		zap.String("clientID", clientID),
		zap.Float64("rate", limits.Rate),
		zap.Int("burst", limits.Burst),
	)

	return limiter
}

func (tb *TokenBucket) GetTokens(clientID string) float64 {
	limiter := tb.getLimiter(clientID)
	return float64(limiter.Tokens())
}

func (tb *TokenBucket) GetBurst(clientID string) int {
	limits := tb.GetClientLimits(clientID)
	return limits.Burst
}

func (tb *TokenBucket) GetRate(clientID string) float64 {
	limits := tb.GetClientLimits(clientID)
	return limits.Rate
}
