package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port string `env:"PORT" envDefault:"8080"`
	Env  string `env:"APP_ENV" envDefault:"development"`

	// Database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Redis Settings
	RedisURL string `env:"REDIS_URL,required"`

	// RabbitMQ Settings
	RabbitMQURL string `env:"RABBITMQ_URL,required"`
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
