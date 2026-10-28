package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port        string `env:"PORT,required"`
	Env         string `env:"APP_ENV,required"`
	FrontendURL string `env:"FRONTEND_URL,required"`

	// Database, Redis & RabbitMQ
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`
	RabbitMQURL string `env:"RABBITMQ_URL,required"`

	// SMTP Settings
	SMTPHost      string `env:"SMTP_HOST,required"`
	SMTPPort      int    `env:"SMTP_PORT,required"`
	SMTPUsername  string `env:"SMTP_USERNAME"`
	SMTPPassword  string `env:"SMTP_PASSWORD"`
	SMTPFromEmail string `env:"SMTP_FROM_EMAIL,required"`
	SMTPFromName  string `env:"SMTP_FROM_NAME,required"`
	SMTPSSL       bool   `env:"SMTP_SSL"`

	// JWT Settings
	JWTAccessSecret  string        `env:"JWT_ACCESS_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	JWTAccessExpiry  time.Duration `env:"JWT_ACCESS_EXPIRY,required"`
	JWTRefreshExpiry time.Duration `env:"JWT_REFRESH_EXPIRY,required"`

	// Cookie Settings
	CookieSecret        string        `env:"COOKIE_SECRET,required"`
	CookieDomain        string        `env:"COOKIE_DOMAIN"`
	CookieSecure        bool          `env:"COOKIE_SECURE"`
	CookieSameSite      string        `env:"COOKIE_SAMESITE,required"`
	AccessCookieExpiry  time.Duration `env:"ACCESS_COOKIE_EXPIRY,required"`
	RefreshCookieExpiry time.Duration `env:"REFRESH_COOKIE_EXPIRY,required"`

	// Admin Seeding Credentials
	SeedAdminEmail    string `env:"SEED_ADMIN_EMAIL,required"`
	SeedAdminPassword string `env:"SEED_ADMIN_PASSWORD,required"`
	SeedAdminName     string `env:"SEED_ADMIN_NAME,required"`
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config environment variables: %w", err)
	}

	return &cfg, nil
}
