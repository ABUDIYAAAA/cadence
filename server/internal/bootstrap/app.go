package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/database"
	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/internal/modules/audit"
	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/judging"
	"devclubxnst.com/cadence/internal/modules/notifications"
	"devclubxnst.com/cadence/internal/modules/teams"
	"devclubxnst.com/cadence/internal/modules/tickets"
	"devclubxnst.com/cadence/internal/modules/users"
	"devclubxnst.com/cadence/internal/ratelimit"
	"devclubxnst.com/cadence/pkg/logger"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
)

// Application encapsulates shared resources, background workers, and routing.
type Application struct {
	Config        *config.Config
	DB            *pgxpool.Pool
	Redis         *redis.Client
	Mailer        *mailer.Mailer
	MailQueue     *mailer.Queue
	AuditQueue        *audit.Queue
	NotificationQueue *notifications.Queue
	Limiter           *ratelimit.Limiter
	LoginGuard    *ratelimit.LoginGuard
	Logger        *slog.Logger
	Router        *chi.Mux
	CookieManager *utils.CookieManager
}

// NewApplication boots config, database, redis, rabbitmq queues, structured logger, and module routes.
func NewApplication(ctx context.Context) (*Application, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	appLogger := logger.NewLogger(cfg.IsProduction())

	dbPool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db pool: %w", err)
	}

	redisClient, err := database.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("connect redis client: %w", err)
	}

	// Initialize rate limiter and login brute-force guard
	limiter := ratelimit.NewLimiter(redisClient)
	loginGuard := ratelimit.NewLoginGuard(redisClient)

	// Initialize audit trail repository and RabbitMQ queue
	auditRepo := audit.NewRepository(dbPool)
	auditQueue, err := audit.NewQueue(cfg.RabbitMQURL, auditRepo, appLogger)
	if err != nil {
		appLogger.Warn("RabbitMQ audit queue connection failed; audit logging disabled", slog.Any("error", err))
	} else {
		// Launch 3 background consumer goroutines for audit log ingestion
		auditQueue.StartConsumers(ctx, 3)
	}

	// Initialize mailer template engine and SMTP transport
	templateEngine, err := mailer.NewTemplateEngine()
	if err != nil {
		return nil, fmt.Errorf("init mailer template engine: %w", err)
	}

	mailClient, err := mailer.NewMailer(cfg, templateEngine)
	if err != nil {
		return nil, fmt.Errorf("init mailer: %w", err)
	}

	// Initialize RabbitMQ mail queue producer and background consumer pool
	mailQueue, err := mailer.NewQueue(cfg.RabbitMQURL, mailClient)
	if err != nil {
		appLogger.Warn("RabbitMQ mail queue connection failed; mail queue disabled", slog.Any("error", err))
	} else {
		// Launch 3 background consumer goroutines in separate threads of execution
		mailQueue.StartConsumers(ctx, 3)
	}

	cookieManager := utils.NewCookieManager(
		cfg.CookieSecret,
		cfg.CookieDomain,
		cfg.CookieSecure,
		cfg.CookieSameSite,
	)

	router := chi.NewRouter()

	// Base middlewares with structured slog logging
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(logger.HTTPMiddleware(appLogger))
	router.Use(middleware.Recoverer)

	// HTTP RED Metrics Middleware
	router.Use(metrics.Middleware())

	// Secure headers middleware
	router.Use(secureHeadersMiddleware(cfg.IsProduction()))

	// Cryptographically signed Device ID injection (isolates devices behind NAT)
	router.Use(ratelimit.DeviceIDMiddleware(cookieManager))

	// Stateless Redis-backed sliding window rate limiter
	router.Use(ratelimit.RateLimitMiddleware(limiter, cfg, cookieManager))

	// Cross-Origin Resource Sharing using frontend URL from .env
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"env":    cfg.Env,
		})
	})

	// Prometheus metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	// Register PostgreSQL connection pool stats
	metrics.RegisterDBStats(dbPool)

	// Auth module initialization with instant Redis blocklist verification and audit recording
	authRepo := auth.NewRepository(dbPool)
	authService := auth.NewService(authRepo, cfg, mailQueue, auditQueue)
	authHandler := auth.NewHandler(authService, cookieManager, cfg, loginGuard)
	authMiddleware := auth.AuthMiddleware(cfg, cookieManager, redisClient)

	// Users module initialization
	usersRepo := users.NewRepository(dbPool)
	usersService := users.NewService(usersRepo, cfg, mailQueue, redisClient, auditQueue)
	usersHandler := users.NewHandler(usersService)

	// Teams module initialization
	teamsRepo := teams.NewRepository(dbPool)
	teamsService := teams.NewService(teamsRepo, auditQueue)
	teamsHandler := teams.NewHandler(teamsService)

	// Audit module initialization
	auditHandler := audit.NewHandler(auditRepo)

	// Tickets & Attendance module initialization
	ticketsRepo := tickets.NewRepository(dbPool)
	ticketsService := tickets.NewService(ticketsRepo, cfg, mailQueue, auditQueue)
	ticketsHandler := tickets.NewHandler(ticketsService)

	// Judging, Scoring & Queue Tracking module initialization
	judgingRepo := judging.NewRepository(dbPool)
	judgingService := judging.NewService(judgingRepo, auditQueue)
	judgingHandler := judging.NewHandler(judgingService)

	// Notifications & Web Push module initialization
	notifRepo := notifications.NewRepository(dbPool)
	notifQueue, err := notifications.NewQueue(cfg, notifRepo, appLogger)
	if err != nil {
		appLogger.Warn("RabbitMQ push notification queue connection failed; push worker disabled", slog.Any("error", err))
	} else {
		// Launch 3 background consumer goroutines for Web Push dispatch
		notifQueue.StartConsumers(ctx, 3)
	}
	notifService := notifications.NewService(notifRepo, notifQueue, cfg, auditQueue)
	notifHandler := notifications.NewHandler(notifService)

	// Mount API v1
	router.Route("/api/v1", func(r chi.Router) {
		auth.RegisterRoutes(r, authHandler, authMiddleware)
		users.RegisterRoutes(r, usersHandler, authMiddleware)
		teams.RegisterRoutes(r, teamsHandler, authMiddleware)
		tickets.RegisterRoutes(r, ticketsHandler, authMiddleware)
		judging.RegisterRoutes(r, judgingHandler, authMiddleware)
		notifications.RegisterRoutes(r, notifHandler, authMiddleware)
		audit.RegisterRoutes(r, auditHandler, authMiddleware)
	})

	return &Application{
		Config:            cfg,
		DB:                dbPool,
		Redis:             redisClient,
		Mailer:            mailClient,
		MailQueue:         mailQueue,
		AuditQueue:        auditQueue,
		NotificationQueue: notifQueue,
		Limiter:       limiter,
		LoginGuard:    loginGuard,
		Logger:        appLogger,
		Router:        router,
		CookieManager: cookieManager,
	}, nil
}

// secureHeadersMiddleware sets industry-standard HTTP security headers.
func secureHeadersMiddleware(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
