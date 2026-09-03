package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
	CORS     CORSConfig     `yaml:"cors"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port            string `yaml:"port"`
	Mode            string `yaml:"mode"`             // debug, release, test
	ShutdownTimeout int    `yaml:"shutdown_timeout"` // seconds
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Driver   string `yaml:"driver"`    // sqlite, mysql
	DSN      string `yaml:"dsn"`       // connection string
	LogLevel string `yaml:"log_level"` // silent, error, warn, info
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret      string `yaml:"secret"`
	TokenExpiry int    `yaml:"token_expiry"` // hours, 0 = no expiry
}

// LogConfig holds structured logger settings.
type LogConfig struct {
	Level    string `yaml:"level"`    // debug, info, warn, error
	Encoding string `yaml:"encoding"` // json, console
	Output   string `yaml:"output"`   // stdout, file path
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"`
}

// Default returns a config with sensible defaults for development.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            "8080",
			Mode:            "debug",
			ShutdownTimeout: 10,
		},
	Database: DatabaseConfig{
		Driver:   "mysql",
		DSN:      "root:@tcp(127.0.0.1:3306)/zoek?charset=utf8mb4&parseTime=true&loc=Local",
		LogLevel: "warn",
	},
		JWT: JWTConfig{
			Secret:      "zoek-dev-secret-change-in-production",
			TokenExpiry: 168, // 7 days
		},
		Log: LogConfig{
			Level:    "info",
			Encoding: "console",
			Output:   "stdout",
		},
		CORS: CORSConfig{
			AllowOrigins: []string{"*"},
		},
	}
}

// Load reads a YAML config file and merges environment variable overrides.
// If path is empty, returns Default() with env overrides applied.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config yaml: %w", err)
		}
	}

	// Environment variable overrides (highest priority)
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides overrides config fields from environment variables.
// Convention: ZOEK_<SECTION>_<FIELD>
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ZOEK_SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("ZOEK_SERVER_MODE"); v != "" {
		cfg.Server.Mode = v
	}
	if v := os.Getenv("ZOEK_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("ZOEK_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("ZOEK_DB_LOG_LEVEL"); v != "" {
		cfg.Database.LogLevel = v
	}
	if v := os.Getenv("ZOEK_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("ZOEK_JWT_EXPIRY"); v != "" {
		// simple int parse without importing strconv
		var hours int
		for _, c := range v {
			hours = hours*10 + int(c-'0')
		}
		cfg.JWT.TokenExpiry = hours
	}
	if v := os.Getenv("ZOEK_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("ZOEK_LOG_ENCODING"); v != "" {
		cfg.Log.Encoding = v
	}
	if v := os.Getenv("ZOEK_CORS_ORIGINS"); v != "" {
		cfg.CORS.AllowOrigins = strings.Split(v, ",")
	}
}

// DSNString returns the database connection string.
func (c *DatabaseConfig) DSNString() string {
	return c.DSN
}
