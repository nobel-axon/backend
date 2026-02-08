// Package config provides configuration loading for axon-server.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the server.
type Config struct {
	// Server settings
	Port string `yaml:"port"`

	// Database settings
	Database DatabaseConfig `yaml:"database"`

	// Chief settings
	Chief ChiefConfig `yaml:"chief"`

	// Poller settings
	Poller PollerConfig `yaml:"poller"`

	// WebSocket settings
	WebSocket WebSocketConfig `yaml:"websocket"`

	// CORS settings
	CORS CORSConfig `yaml:"cors"`

	// Rate limit settings
	RateLimit RateLimitConfig `yaml:"rateLimit"`

	// Lobby settings
	Lobby LobbyConfig `yaml:"lobby"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Name            string        `yaml:"name"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"sslMode"`
	MaxOpenConns    int           `yaml:"maxOpenConns"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"`
}

// ChiefConfig holds settings for communicating with axon-chief.
type ChiefConfig struct {
	URL            string `yaml:"url"`
	TimeoutMs      int    `yaml:"timeoutMs"`
	RetryCount     int    `yaml:"retryCount"`
	RetryDelayMs   int    `yaml:"retryDelayMs"`
	InternalSecret string `yaml:"internalSecret"`
}

// PollerConfig holds poller settings.
type PollerConfig struct {
	Enabled       bool          `yaml:"enabled"`
	IntervalMs    int           `yaml:"intervalMs"`
	BatchSize     int           `yaml:"batchSize"`
}

// WebSocketConfig holds WebSocket settings.
type WebSocketConfig struct {
	ReadBufferSize  int           `yaml:"readBufferSize"`
	WriteBufferSize int           `yaml:"writeBufferSize"`
	PingInterval    time.Duration `yaml:"pingInterval"`
	PongTimeout     time.Duration `yaml:"pongTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowedOrigins"`
	AllowedMethods []string `yaml:"allowedMethods"`
	AllowedHeaders []string `yaml:"allowedHeaders"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled          bool `yaml:"enabled"`
	RequestsPerMin   int  `yaml:"requestsPerMin"`
	BurstSize        int  `yaml:"burstSize"`
	CleanupIntervalS int  `yaml:"cleanupIntervalS"`
}

// LobbyConfig holds lobby settings for pre-payment interest signaling.
type LobbyConfig struct {
	Enabled          bool `yaml:"enabled"`
	MinPlayers       int  `yaml:"minPlayers"`
	MaxPlayers       int  `yaml:"maxPlayers"`
	StaleTimeoutS    int  `yaml:"staleTimeoutS"`
	ReadyCooldownS   int  `yaml:"readyCooldownS"`
	ReadyWindowS     int  `yaml:"readyWindowS"`
	CleanupIntervalS int  `yaml:"cleanupIntervalS"`
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Port: "8080",
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			Name:            "axon",
			User:            "axon",
			Password:        "",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Chief: ChiefConfig{
			URL:            "http://localhost:9100",
			TimeoutMs:      5000,
			RetryCount:     3,
			RetryDelayMs:   500,
			InternalSecret: "",
		},
		Poller: PollerConfig{
			Enabled:    true,
			IntervalMs: 500,
			BatchSize:  100,
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    30 * time.Second,
			PongTimeout:     60 * time.Second,
			WriteTimeout:    10 * time.Second,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		},
		RateLimit: RateLimitConfig{
			Enabled:          true,
			RequestsPerMin:   60,
			BurstSize:        10,
			CleanupIntervalS: 60,
		},
		Lobby: LobbyConfig{
			Enabled:          true,
			MinPlayers:       2,
			MaxPlayers:       8,
			StaleTimeoutS:    300,
			ReadyCooldownS:   30,
			ReadyWindowS:     120,
			CleanupIntervalS: 10,
		},
	}
}

// Load loads configuration from a YAML file and environment variables.
// Environment variables take precedence over file values.
func Load(path string) (*Config, error) {
	cfg := Default()

	// Load from file if provided
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}

	// Database
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = port
		}
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_SSL_MODE"); v != "" {
		cfg.Database.SSLMode = v
	}

	// Chief
	if v := os.Getenv("CHIEF_URL"); v != "" {
		cfg.Chief.URL = v
	}
	if v := os.Getenv("INTERNAL_SECRET"); v != "" {
		cfg.Chief.InternalSecret = v
	}

	// Poller
	if v := os.Getenv("POLLER_ENABLED"); v != "" {
		cfg.Poller.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("POLLER_INTERVAL_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			cfg.Poller.IntervalMs = ms
		}
	}

	// Lobby
	if v := os.Getenv("LOBBY_ENABLED"); v != "" {
		cfg.Lobby.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("LOBBY_MIN_PLAYERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Lobby.MinPlayers = n
		}
	}
	if v := os.Getenv("LOBBY_MAX_PLAYERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Lobby.MaxPlayers = n
		}
	}

	// CORS
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CORS.AllowedOrigins = strings.Split(v, ",")
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + strconv.Itoa(c.Port) +
		" dbname=" + c.Name +
		" user=" + c.User +
		" password=" + c.Password +
		" sslmode=" + c.SSLMode
}

// GetTimeout returns the Chief timeout as time.Duration.
func (c *ChiefConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutMs) * time.Millisecond
}

// GetRetryDelay returns the Chief retry delay as time.Duration.
func (c *ChiefConfig) GetRetryDelay() time.Duration {
	return time.Duration(c.RetryDelayMs) * time.Millisecond
}

// GetInterval returns the poller interval as time.Duration.
func (c *PollerConfig) GetInterval() time.Duration {
	return time.Duration(c.IntervalMs) * time.Millisecond
}
