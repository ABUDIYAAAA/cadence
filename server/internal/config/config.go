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

	// Web Push VAPID Settings
	VAPIDPublicKey  string `env:"VAPID_PUBLIC_KEY,required"`
	VAPIDPrivateKey string `env:"VAPID_PRIVATE_KEY,required"`
	VAPIDSubject    string `env:"VAPID_SUBJECT,required"`

	// Telemetry & Metrics Scrape Authentication
	MetricsScrapeToken string `env:"METRICS_SCRAPE_TOKEN"`

	// Global Rate Limiting (sliding window per-minute request quota)
	GlobalRateLimit int64 `env:"GLOBAL_RATE_LIMIT" envDefault:"500"`
	UnauthRateLimit int64 `env:"UNAUTH_RATE_LIMIT" envDefault:"1000"`

	// Event-day sign-in traffic: roughly a thousand people on one campus network hit /login, /auth/me,
	// forgot/reset-password together. These raise the shared per-IP ceilings accordingly.
	CampusIPRateLimit   int64 `env:"CAMPUS_IP_RATE_LIMIT" envDefault:"30000"`  // unauthenticated requests per minute, per IP
	LoginIPFailureLimit int64 `env:"LOGIN_IP_FAILURE_LIMIT" envDefault:"3000"` // failed logins per 5 minutes, per IP

	// Background workers that send queued emails (password resets, passes). Raise if SMTP allows.
	MailWorkers int `env:"MAIL_WORKERS" envDefault:"8"`

	// Default maximum members per team; admins can override it in the UI or per team.
	DefaultMaxTeamSize int `env:"DEFAULT_MAX_TEAM_SIZE" envDefault:"4"`

	// Email domains of internal attendees (e.g. the college). Anyone else is external:
	// they receive their QR by email and are the only ones who exit-scan.
	InternalEmailDomains []string `env:"INTERNAL_EMAIL_DOMAINS" envSeparator:","`

	// Scheduled Database Backups to Cloudflare R2 (disabled unless R2_BUCKET is set)
	R2AccountID       string        `env:"R2_ACCOUNT_ID"`
	R2AccessKeyID     string        `env:"R2_ACCESS_KEY_ID"`
	R2SecretAccessKey string        `env:"R2_SECRET_ACCESS_KEY"`
	R2Bucket          string        `env:"R2_BUCKET"`
	R2Endpoint        string        `env:"R2_ENDPOINT"`
	BackupInterval    time.Duration `env:"BACKUP_INTERVAL" envDefault:"30m"`
	BackupPrefix      string        `env:"BACKUP_PREFIX" envDefault:"db-backups"`
	BackupRetention   time.Duration `env:"BACKUP_RETENTION"`
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// BackupEnabled reports whether R2 credentials are present for scheduled backups.
func (c *Config) BackupEnabled() bool {
	return c.R2Bucket != "" && c.R2AccessKeyID != "" && c.R2SecretAccessKey != "" &&
		(c.R2Endpoint != "" || c.R2AccountID != "")
}

// BackupEndpoint returns the R2 S3 endpoint host, derived from the account ID unless overridden.
func (c *Config) BackupEndpoint() string {
	if c.R2Endpoint != "" {
		return c.R2Endpoint
	}
	return c.R2AccountID + ".r2.cloudflarestorage.com"
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config environment variables: %w", err)
	}

	return &cfg, nil
}
