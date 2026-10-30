package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/metrics"
)

const (
	dumpTimeout   = 10 * time.Minute
	uploadTimeout = 10 * time.Minute
)

// Service periodically dumps PostgreSQL with pg_dump and uploads the archive to Cloudflare R2.
type Service struct {
	dbURL     string
	bucket    string
	prefix    string
	interval  time.Duration
	retention time.Duration
	client    *minio.Client
	logger    *slog.Logger

	mu sync.Mutex // serialises runs so a slow backup never overlaps the next tick
}

// New builds a backup service from config. It returns nil, nil when R2 is not configured.
func New(cfg *config.Config, logger *slog.Logger) (*Service, error) {
	if !cfg.BackupEnabled() {
		return nil, nil
	}
	if cfg.BackupInterval <= 0 {
		return nil, errors.New("BACKUP_INTERVAL must be positive")
	}

	client, err := minio.New(cfg.BackupEndpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, ""),
		Secure: true,
		Region: "auto",
	})
	if err != nil {
		return nil, fmt.Errorf("init r2 client: %w", err)
	}

	return &Service{
		dbURL:     cfg.DatabaseURL,
		bucket:    cfg.R2Bucket,
		prefix:    strings.Trim(cfg.BackupPrefix, "/"),
		interval:  cfg.BackupInterval,
		retention: cfg.BackupRetention,
		client:    client,
		logger:    logger,
	}, nil
}

// Start runs a backup immediately, then on every interval until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	metrics.BackupEnabled.Set(1)
	go func() {
		defer metrics.BackupEnabled.Set(0)

		s.runLogged(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runLogged(ctx)
			}
		}
	}()
	s.logger.Info("database backups enabled",
		slog.String("bucket", s.bucket), slog.Duration("interval", s.interval))
}

func (s *Service) runLogged(ctx context.Context) {
	if err := s.Run(ctx); err != nil && ctx.Err() == nil {
		s.logger.Error("database backup failed", slog.Any("error", err))
	}
}

// Run performs a single dump-and-upload cycle and records metrics.
func (s *Service) Run(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics.BackupLastRunTimestamp.SetToCurrentTime()

	started := time.Now().UTC()
	file, err := os.CreateTemp("", "cadence-backup-*.dump")
	if err != nil {
		metrics.BackupRunsTotal.WithLabelValues("dump_failed").Inc()
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	dumpStart := time.Now()
	if err := s.dump(ctx, file); err != nil {
		metrics.BackupRunsTotal.WithLabelValues("dump_failed").Inc()
		return err
	}
	metrics.BackupDuration.WithLabelValues("dump").Observe(time.Since(dumpStart).Seconds())

	info, err := file.Stat()
	if err != nil {
		metrics.BackupRunsTotal.WithLabelValues("dump_failed").Inc()
		return fmt.Errorf("stat dump: %w", err)
	}

	key := path.Join(s.prefix, fmt.Sprintf("cadence-%s.dump", started.Format("20060102-150405Z")))
	if _, err := file.Seek(0, 0); err != nil {
		metrics.BackupRunsTotal.WithLabelValues("upload_failed").Inc()
		return fmt.Errorf("rewind dump: %w", err)
	}

	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	uploadStart := time.Now()
	_, err = s.client.PutObject(uploadCtx, s.bucket, key, file, info.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		metrics.BackupRunsTotal.WithLabelValues("upload_failed").Inc()
		return fmt.Errorf("upload %s: %w", key, err)
	}
	metrics.BackupDuration.WithLabelValues("upload").Observe(time.Since(uploadStart).Seconds())

	metrics.BackupRunsTotal.WithLabelValues("success").Inc()
	metrics.BackupLastSuccessTimestamp.SetToCurrentTime()
	metrics.BackupLastSizeBytes.Set(float64(info.Size()))
	s.logger.Info("database backup uploaded",
		slog.String("key", key), slog.Int64("bytes", info.Size()),
		slog.Duration("elapsed", time.Since(started)))

	s.prune(ctx)
	return nil
}

// dump writes a custom-format (compressed) pg_dump archive into out.
// Credentials go through PGPASSWORD so they never show up in the process list.
func (s *Service) dump(ctx context.Context, out *os.File) error {
	u, err := url.Parse(s.dbURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", errors.New("invalid DATABASE_URL"))
	}
	password, _ := u.User.Password()
	q := u.Query()
	sslmode := q.Get("sslmode")
	if sslmode == "" {
		sslmode = "prefer"
	}

	ctx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pg_dump",
		"--format=custom", "--no-owner", "--no-privileges",
		"--host", u.Hostname(), "--port", portOrDefault(u),
		"--username", u.User.Username(),
		"--dbname", strings.TrimPrefix(u.Path, "/"),
		"--file", out.Name(),
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password, "PGSSLMODE="+sslmode)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_dump: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func portOrDefault(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	return "5432"
}

// prune deletes backups older than the retention window. No-op when retention is unset.
func (s *Service) prune(ctx context.Context) {
	if s.retention <= 0 {
		return
	}
	cutoff := time.Now().Add(-s.retention)
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.prefix + "/"}) {
		if obj.Err != nil {
			s.logger.Warn("backup prune: list failed", slog.Any("error", obj.Err))
			return
		}
		if !strings.HasSuffix(obj.Key, ".dump") || !obj.LastModified.Before(cutoff) {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			s.logger.Warn("backup prune: delete failed", slog.String("key", obj.Key), slog.Any("error", err))
		}
	}
}
