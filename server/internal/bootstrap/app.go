package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/database"
	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
)

// Application encapsulates shared resources, background workers, and routing.
type Application struct {
	Config        *config.Config
	DB            *pgxpool.Pool
	Redis         *redis.Client
	Mailer        *mailer.Mailer
	MailQueue     *mailer.Queue
	Router        *chi.Mux
	CookieManager *utils.CookieManager
}

// NewApplication boots config, database, redis, rabbitmq mailer, middleware, and module routes.
func NewApplication(ctx context.Context) (*Application, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dbPool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db pool: %w", err)
	}

	redisClient, err := database.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("connect redis client: %w", err)
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
		log.Printf("[WARN] RabbitMQ not reachable (%v). Mail queue disabled.", err)
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

	// Base middlewares
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Secure headers middleware
	router.Use(secureHeadersMiddleware(cfg.IsProduction()))

	// Cross-Origin Resource Sharing using frontend URL from .env
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
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

	// Auth module initialization
	authRepo := auth.NewRepository(dbPool)
	authService := auth.NewService(authRepo, cfg, mailQueue)
	authHandler := auth.NewHandler(authService, cookieManager, cfg)
	authMiddleware := auth.AuthMiddleware(cfg, cookieManager)

	// Mount API v1
	router.Route("/api/v1", func(r chi.Router) {
		auth.RegisterRoutes(r, authHandler, authMiddleware)
	})

	return &Application{
		Config:        cfg,
		DB:            dbPool,
		Redis:         redisClient,
		Mailer:        mailClient,
		MailQueue:     mailQueue,
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
