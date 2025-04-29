package app

import (
	"fmt"
	"net/http"

	"CloudBalancer/config"
	"CloudBalancer/internal/load_balancer"
	"CloudBalancer/internal/rate_limiter"
	"CloudBalancer/internal/transport/http/router"
	"CloudBalancer/pkg/logger"

	"go.uber.org/zap"
)

type App struct {
	config       *config.Config
	logger       *logger.Logger
	router       *router.Router
	loadBalancer load_balancer.LoadBalancer
	rateLimiter  rate_limiter.RateLimiter
}

func NewApp(config *config.Config) (*App, error) {
	log, err := logger.NewLogger(config.Logging.Environment)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	lb, err := load_balancer.NewLoadBalancer(config, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize load balancer: %w", err)
	}

	var rl rate_limiter.RateLimiter
	if config.RateLimit.Enabled {
		rl = rate_limiter.NewTokenBucket(
			config.RateLimit.DefaultRate,
			config.RateLimit.DefaultBurst,
			log.Logger,
		)
		log.Logger.Info("Rate limiter initialized",
			zap.Float64("defaultRate", config.RateLimit.DefaultRate),
			zap.Int("defaultBurst", config.RateLimit.DefaultBurst),
		)
	} else {
		log.Logger.Info("Rate limiting is disabled")
		rl = rate_limiter.NewTokenBucket(1000000, 1000000, log.Logger)
	}

	r := router.NewRouter(log.Logger, lb, rl)
	r.SetupRoutes()

	return &App{
		config:       config,
		logger:       log,
		router:       r,
		loadBalancer: lb,
		rateLimiter:  rl,
	}, nil
}

func (a *App) Router() http.Handler {
	return a.router
}
