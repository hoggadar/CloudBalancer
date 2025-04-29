package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var SupportedBalancingMethods = []string{
	"RoundRobin",
}

type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	LoadBalancer LoadBalancerConfig `mapstructure:"loadBalancer"`
	Backends     []BackendConfig    `mapstructure:"backends"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	RateLimit    RateLimitConfig    `mapstructure:"rateLimit"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type LoadBalancerConfig struct {
	Method              string        `mapstructure:"method"`
	HealthCheckInterval time.Duration `mapstructure:"healthCheckInterval"`
}

type BackendConfig struct {
	ID             string        `mapstructure:"id"`
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	ConnectTimeout time.Duration `mapstructure:"connectTimeout"`
	ReadTimeout    time.Duration `mapstructure:"readTimeout"`
	MaxConnection  int           `mapstructure:"maxConnection"`
	Enabled        bool          `mapstructure:"enabled"`
}

type LoggingConfig struct {
	Environment string `mapstructure:"environment"`
	Level       string `mapstructure:"level"`
}

type RateLimitConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	DefaultRate  float64 `mapstructure:"defaultRate"`
	DefaultBurst int     `mapstructure:"defaultBurst"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())

	viper.SetDefault("loadBalancer.method", "RoundRobin")
	viper.SetDefault("loadBalancer.healthCheckInterval", "10s")

	viper.SetDefault("rateLimit.enabled", true)
	viper.SetDefault("rateLimit.defaultRate", 100.0)
	viper.SetDefault("rateLimit.defaultBurst", 50)

	viper.RegisterAlias("loadBalancer.healthCheckInterval", "loadBalancer.healthCheckInterval")
	viper.RegisterAlias("backends.connectTimeout", "backends.connectTimeout")
	viper.RegisterAlias("backends.readTimeout", "backends.readTimeout")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateConfig(config *Config) error {
	validMethod := false
	for _, method := range SupportedBalancingMethods {
		if config.LoadBalancer.Method == method {
			validMethod = true
			break
		}
	}
	if !validMethod {
		return fmt.Errorf("unsupported balancing method: %s. Supported methods: %v",
			config.LoadBalancer.Method, SupportedBalancingMethods)
	}

	if len(config.Backends) == 0 {
		return fmt.Errorf("no backends configured")
	}

	enabledBackends := 0
	for i, backend := range config.Backends {
		if backend.ID == "" {
			return fmt.Errorf("backend #%d has empty ID", i)
		}
		if backend.Enabled {
			enabledBackends++
		}
	}

	if enabledBackends == 0 {
		return fmt.Errorf("no enabled backends configured")
	}

	if config.RateLimit.Enabled {
		if config.RateLimit.DefaultRate <= 0 {
			return fmt.Errorf("rate limit default rate must be positive, got %f", config.RateLimit.DefaultRate)
		}
		if config.RateLimit.DefaultBurst <= 0 {
			return fmt.Errorf("rate limit default burst must be positive, got %d", config.RateLimit.DefaultBurst)
		}
	}

	return nil
}
