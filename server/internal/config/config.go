package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port        string `env:"PORT" envDefault:"8080"`
	Env         string `env:"APP_ENV" envDefault:"development"`
	FrontendURL string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`

	// Database, Redis & RabbitMQ
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`
	RabbitMQURL string `env:"RABBITMQ_URL,required"`

	// JWT Settings
	JWTAccessSecret  string        `env:"JWT_ACCESS_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	JWTAccessExpiry  time.Duration `env:"JWT_ACCESS_EXPIRY" envDefault:"15m"`
	JWTRefreshExpiry time.Duration `env:"JWT_REFRESH_EXPIRY" envDefault:"168h"`

	// Cookie Settings
	CookieSecret        string        `env:"COOKIE_SECRET,required"`
	CookieDomain        string        `env:"COOKIE_DOMAIN" envDefault:""`
	CookieSecure        bool          `env:"COOKIE_SECURE" envDefault:"false"`
	CookieSameSite      string        `env:"COOKIE_SAMESITE" envDefault:"lax"`
	AccessCookieExpiry  time.Duration `env:"ACCESS_COOKIE_EXPIRY" envDefault:"15m"`
	RefreshCookieExpiry time.Duration `env:"REFRESH_COOKIE_EXPIRY" envDefault:"168h"`

	// Admin Seeding Credentials
	SeedAdminEmail    string `env:"SEED_ADMIN_EMAIL" envDefault:"admin@cadence.local"`
	SeedAdminPassword string `env:"SEED_ADMIN_PASSWORD" envDefault:"AdminPassword123!"`
	SeedAdminName     string `env:"SEED_ADMIN_NAME" envDefault:"System Admin"`
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
