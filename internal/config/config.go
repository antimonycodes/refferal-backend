package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	CORS      CORSConfig
	Admin     AdminConfig
	RateLimit RateLimitConfig
	Paystack  PaystackConfig
	SMTP      SMTPConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	URL      string
	TestURL  string
	MaxConns int
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	Secret        string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type CORSConfig struct {
	Origin string
}

type AdminConfig struct {
	Email    string
	Password string
	Name     string
}

type RateLimitConfig struct {
	Requests int
	Window   time.Duration
}

type PaystackConfig struct {
	SecretKey string
	BaseURL   string
}

type SMTPConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	FromName    string
	FromEmail   string
	FrontendURL string
}

func Load() (*Config, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	accessExpiry, _ := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"))
	rateWindow, _ := time.ParseDuration(getEnv("RATE_LIMIT_WINDOW", "1m"))

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
			Env:  getEnv("ENV", "development"),
		},
		Database: DatabaseConfig{
			URL:      getEnv("DATABASE_URL", ""),
			TestURL:  getEnv("DATABASE_TEST_URL", ""),
			MaxConns: getEnvInt("DB_MAX_CONNS", 25),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", ""),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		},
		CORS: CORSConfig{
			Origin: getEnv("CORS_ORIGIN", "http://localhost:5173"),
		},
		Admin: AdminConfig{
			Email:    getEnv("ADMIN_EMAIL", "admin@cirvee.com"),
			Password: getEnv("ADMIN_PASSWORD", ""),
			Name:     getEnv("ADMIN_NAME", "Super Admin"),
		},
		RateLimit: RateLimitConfig{
			Requests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
			Window:   rateWindow,
		},
		Paystack: PaystackConfig{
			SecretKey: getEnv("PAYSTACK_SECRET_KEY", ""),
			BaseURL:   getEnv("PAYSTACK_BASE_URL", "https://api.paystack.co"),
		},
		SMTP: SMTPConfig{
			Host:        getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:        getEnvInt("SMTP_PORT", 587),
			User:        getEnv("SMTP_USER", ""),
			Password:    getEnv("SMTP_PASSWORD", ""),
			FromName:    getEnv("SMTP_FROM_NAME", "Cirvee"),
			FromEmail:   getEnv("SMTP_FROM_EMAIL", "noreply@cirvee.com"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		},
	}

	// Validate critical configuration
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.JWT.RefreshSecret == "" {
		return nil, fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		var result int
		_, err := fmt.Sscanf(value, "%d", &result)
		if err == nil {
			return result
		}
	}
	return fallback
}
