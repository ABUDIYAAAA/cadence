package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"devclubxnst.com/cadence/pkg/audit"
)

const (
	settingInternalDomains = "internal_email_domains"
	settingsCacheTTL       = 15 * time.Second
)

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

type TicketSettings struct {
	InternalEmailDomains []string `json:"internal_email_domains"`
	// Source is "database" once an admin has saved a list, otherwise "environment".
	Source string `json:"source"`
}

type UpdateTicketSettingsRequest struct {
	InternalEmailDomains []string `json:"internal_email_domains"`
}

// settingsCache keeps scans from hitting the database for the domain list on every request.
type settingsCache struct {
	mu      sync.RWMutex
	domains []string
	source  string
	expires time.Time
}

var ErrInvalidDomain = errors.New("invalid email domain")

func (r *Repository) GetSetting(ctx context.Context, key string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := r.db.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get setting: %w", err)
	}
	return raw, nil
}

func (r *Repository) SetSetting(ctx context.Context, key string, value any, updatedBy string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO app_settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`,
		key, raw, updatedBy)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

// normalizeDomains lower-cases, strips a leading "@", validates and de-duplicates.
func normalizeDomains(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
		if d == "" {
			continue
		}
		if !domainPattern.MatchString(d) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidDomain, d)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out, nil
}

// loadSettings returns the saved domain list, or the INTERNAL_EMAIL_DOMAINS fallback when none was saved.
func (s *Service) loadSettings(ctx context.Context) (TicketSettings, error) {
	raw, err := s.repo.GetSetting(ctx, settingInternalDomains)
	if err != nil {
		return TicketSettings{}, err
	}
	if raw == nil {
		domains, _ := normalizeDomains(s.cfg.InternalEmailDomains)
		return TicketSettings{InternalEmailDomains: domains, Source: "environment"}, nil
	}
	var domains []string
	if err := json.Unmarshal(raw, &domains); err != nil {
		return TicketSettings{}, fmt.Errorf("decode setting: %w", err)
	}
	return TicketSettings{InternalEmailDomains: domains, Source: "database"}, nil
}

// InternalDomains returns the effective internal domain list (cached briefly).
func (s *Service) InternalDomains(ctx context.Context) ([]string, error) {
	s.cache.mu.RLock()
	if time.Now().Before(s.cache.expires) {
		d := s.cache.domains
		s.cache.mu.RUnlock()
		return d, nil
	}
	s.cache.mu.RUnlock()

	cur, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	s.cache.mu.Lock()
	s.cache.domains, s.cache.source, s.cache.expires = cur.InternalEmailDomains, cur.Source, time.Now().Add(settingsCacheTTL)
	s.cache.mu.Unlock()
	return cur.InternalEmailDomains, nil
}

func (s *Service) GetSettings(ctx context.Context) (*TicketSettings, error) {
	cur, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if cur.InternalEmailDomains == nil {
		cur.InternalEmailDomains = []string{}
	}
	return &cur, nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateTicketSettingsRequest, actorID, actorEmail, actorRole, ip, ua string) (*TicketSettings, error) {
	domains, err := normalizeDomains(req.InternalEmailDomains)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetSetting(ctx, settingInternalDomains, domains, actorID); err != nil {
		return nil, err
	}
	s.cache.mu.Lock()
	s.cache.expires = time.Time{} // next read reloads
	s.cache.mu.Unlock()

	s.recordAudit(ctx, audit.ActionTicketSettingsUpdate, audit.TargetTicket, nil,
		fmt.Sprintf("%s set internal email domains to [%s]", actorEmail, strings.Join(domains, ", ")),
		&actorID, &actorEmail, &actorRole, map[string]any{"internal_email_domains": domains}, &ip, &ua)
	return &TicketSettings{InternalEmailDomains: domains, Source: "database"}, nil
}
