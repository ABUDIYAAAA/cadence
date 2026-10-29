package bootstrap

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/internal/modules/audit"
	"devclubxnst.com/cadence/internal/modules/notifications"
	"devclubxnst.com/cadence/pkg/utils"
)

// HealthHandler returns an HTTP handler that inspects PostgreSQL, Redis, and RabbitMQ connectivity.
// If any dependency is unreachable, it marks the status as "degraded" and returns HTTP 200 without
// crashing, terminating, or closing down the application server.
func HealthHandler(
	cfg *config.Config,
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	auditQueue *audit.Queue,
	mailQueue *mailer.Queue,
	notifQueue *notifications.Queue,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pgStatus := "up"
		if dbPool != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := dbPool.Ping(pingCtx); err != nil {
				pgStatus = "down"
			}
		} else {
			pgStatus = "down"
		}

		redisStatus := "up"
		if redisClient != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := redisClient.Ping(pingCtx).Err(); err != nil {
				redisStatus = "down"
			}
		} else {
			redisStatus = "down"
		}

		rabbitStatus := "up"
		auditUp := auditQueue != nil && auditQueue.IsHealthy()
		mailUp := mailQueue != nil && mailQueue.IsHealthy()
		notifUp := notifQueue != nil && notifQueue.IsHealthy()

		if !auditUp && !mailUp && !notifUp {
			rabbitStatus = "down"
		} else if !auditUp || !mailUp || !notifUp {
			rabbitStatus = "degraded"
		}

		overallStatus := "ok"
		if pgStatus != "up" || redisStatus != "up" || rabbitStatus != "up" {
			overallStatus = "degraded"
		}

		utils.JSON(w, http.StatusOK, map[string]string{
			"status":   overallStatus,
			"env":      cfg.Env,
			"postgres": pgStatus,
			"redis":    redisStatus,
			"rabbitmq": rabbitStatus,
		})
	}
}
