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

	"devclubxnst.com/cadence/internal/mailer"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/utils"
)

const (
	settingInternalDomains = "internal_email_domains"
	settingEmailSubject    = "ticket_email_subject"
	settingEmailHTML       = "ticket_email_html"
	settingTicketsReleased = "tickets_released"
	settingsCacheTTL       = 15 * time.Second
)

var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

type TicketSettings struct {
	InternalEmailDomains []string `json:"internal_email_domains"`
	ExcludedDomains      []string `json:"excluded_domains"`
	EmailSubject         string   `json:"email_subject"`
	EmailHTML            string   `json:"email_html"`
	TicketsReleased      bool     `json:"tickets_released"`
	// Source is "database" once an admin has saved a domain list, otherwise "environment".
	Source string `json:"source"`
}

type UpdateTicketSettingsRequest struct {
	InternalEmailDomains []string `json:"internal_email_domains"`
	ExcludedDomains      []string `json:"excluded_domains"`
	EmailSubject         *string  `json:"email_subject"`
	EmailHTML            *string  `json:"email_html"`
	TicketsReleased      *bool    `json:"tickets_released"`
}

type cachedEventSettings struct {
	settings TicketSettings
	expires  time.Time
}

// settingsCache keeps scans and ticket lookups from hitting the database on every request, keyed per event_id.
type settingsCache struct {
	mu      sync.RWMutex
	byEvent map[string]cachedEventSettings
}

var (
	ErrInvalidDomain       = errors.New("invalid email domain")
	ErrInvalidEmailSubject = errors.New("invalid email subject: line breaks are not allowed")
)

func (r *Repository) GetSetting(ctx context.Context, key string) (json.RawMessage, error) {
	eventID := resolveEventID(ctx)
	var raw json.RawMessage
	err := r.db.QueryRow(ctx, `SELECT value FROM app_settings WHERE event_id = $1 AND key = $2`, eventID, key).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get setting: %w", err)
	}
	return raw, nil
}

func (r *Repository) SetSetting(ctx context.Context, key string, value any, updatedBy string) error {
	eventID := resolveEventID(ctx)
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO app_settings (event_id, key, value, updated_by) VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
		ON CONFLICT (event_id, key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`,
		eventID, key, raw, updatedBy)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

// normalizeDomains lower-cases, extracts everything from "@" to the end (e.g. "@domain.com" or "user@domain.com"),
// validates the domain format, and de-duplicates.
func normalizeDomains(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		d := utils.ExtractEmailDomain(raw)
		if d == "" {
			continue
		}
		if !domainPattern.MatchString(d) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidDomain, raw)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out, nil
}

// loadSettings returns the event's saved ticket settings, falling back to INTERNAL_EMAIL_DOMAINS when no domains were saved.
func (s *Service) loadSettings(ctx context.Context) (TicketSettings, error) {
	var out TicketSettings

	rawDomains, err := s.repo.GetSetting(ctx, settingInternalDomains)
	if err != nil {
		return TicketSettings{}, err
	}
	if rawDomains == nil {
		domains, _ := normalizeDomains(s.cfg.InternalEmailDomains)
		if domains == nil {
			domains = []string{}
		}
		out.InternalEmailDomains = domains
		out.ExcludedDomains = domains
		out.Source = "environment"
	} else {
		var domains []string
		if err := json.Unmarshal(rawDomains, &domains); err != nil {
			return TicketSettings{}, fmt.Errorf("decode domain setting: %w", err)
		}
		if domains == nil {
			domains = []string{}
		}
		out.InternalEmailDomains = domains
		out.ExcludedDomains = domains
		out.Source = "database"
	}

	if rawSubj, err := s.repo.GetSetting(ctx, settingEmailSubject); err == nil && rawSubj != nil {
		_ = json.Unmarshal(rawSubj, &out.EmailSubject)
	}
	if rawHTML, err := s.repo.GetSetting(ctx, settingEmailHTML); err == nil && rawHTML != nil {
		_ = json.Unmarshal(rawHTML, &out.EmailHTML)
	}
	if rawRel, err := s.repo.GetSetting(ctx, settingTicketsReleased); err == nil && rawRel != nil {
		_ = json.Unmarshal(rawRel, &out.TicketsReleased)
	}

	return out, nil
}

func (s *Service) invalidateSettingsCache(ctx context.Context) {
	eventID := resolveEventID(ctx)
	s.cache.mu.Lock()
	if s.cache.byEvent != nil {
		delete(s.cache.byEvent, eventID)
	}
	s.cache.mu.Unlock()
}

func (s *Service) cachedSettings(ctx context.Context) (TicketSettings, error) {
	eventID := resolveEventID(ctx)
	s.cache.mu.RLock()
	if s.cache.byEvent != nil {
		if entry, ok := s.cache.byEvent[eventID]; ok && time.Now().Before(entry.expires) {
			s.cache.mu.RUnlock()
			return entry.settings, nil
		}
	}
	s.cache.mu.RUnlock()

	cur, err := s.loadSettings(ctx)
	if err != nil {
		return TicketSettings{}, err
	}
	s.cache.mu.Lock()
	if s.cache.byEvent == nil {
		s.cache.byEvent = make(map[string]cachedEventSettings)
	}
	s.cache.byEvent[eventID] = cachedEventSettings{
		settings: cur,
		expires:  time.Now().Add(settingsCacheTTL),
	}
	s.cache.mu.Unlock()
	return cur, nil
}

// InternalDomains returns the effective internal/excluded domain list for the active event (cached briefly).
func (s *Service) InternalDomains(ctx context.Context) ([]string, error) {
	cur, err := s.cachedSettings(ctx)
	if err != nil {
		return nil, err
	}
	return cur.InternalEmailDomains, nil
}

// AreTicketsReleased reports whether tickets have been released for the active event.
func (s *Service) AreTicketsReleased(ctx context.Context) (bool, error) {
	cur, err := s.cachedSettings(ctx)
	if err != nil {
		return false, err
	}
	return cur.TicketsReleased, nil
}

func (s *Service) setTicketsReleased(ctx context.Context, released bool, actorID string) error {
	if err := s.repo.SetSetting(ctx, settingTicketsReleased, released, actorID); err != nil {
		return err
	}
	s.invalidateSettingsCache(ctx)
	return nil
}

func (s *Service) GetSettings(ctx context.Context) (*TicketSettings, error) {
	cur, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	if cur.InternalEmailDomains == nil {
		cur.InternalEmailDomains = []string{}
	}
	cur.ExcludedDomains = cur.InternalEmailDomains
	return &cur, nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateTicketSettingsRequest, actorID, actorEmail string) (*TicketSettings, error) {
	meta := map[string]any{}

	// Merge InternalEmailDomains and ExcludedDomains if provided
	if req.InternalEmailDomains != nil || req.ExcludedDomains != nil {
		combined := make([]string, 0, len(req.InternalEmailDomains)+len(req.ExcludedDomains))
		combined = append(combined, req.InternalEmailDomains...)
		combined = append(combined, req.ExcludedDomains...)
		domains, err := normalizeDomains(combined)
		if err != nil {
			return nil, err
		}
		if err := s.repo.SetSetting(ctx, settingInternalDomains, domains, actorID); err != nil {
			return nil, err
		}
		meta["excluded_domains"] = domains
	}

	if req.EmailSubject != nil {
		subj := strings.TrimSpace(*req.EmailSubject)
		if err := mailer.ValidateHeaderField(subj); err != nil {
			return nil, ErrInvalidEmailSubject
		}
		if err := s.repo.SetSetting(ctx, settingEmailSubject, subj, actorID); err != nil {
			return nil, err
		}
		meta["email_subject"] = subj
	}

	if req.EmailHTML != nil {
		sanitized := utils.SanitizeEmailHTML(*req.EmailHTML)
		if err := s.repo.SetSetting(ctx, settingEmailHTML, sanitized, actorID); err != nil {
			return nil, err
		}
		meta["email_html_updated"] = sanitized != ""
	}

	if req.TicketsReleased != nil {
		if err := s.repo.SetSetting(ctx, settingTicketsReleased, *req.TicketsReleased, actorID); err != nil {
			return nil, err
		}
		meta["tickets_released"] = *req.TicketsReleased
	}

	s.invalidateSettingsCache(ctx)

	updated, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	if s.auditQueue != nil {
		s.auditQueue.RecordAction(ctx, audit.ActionTicketSettingsUpdate, audit.TargetTicket, nil,
			fmt.Sprintf("%s updated ticket settings (excluded domains: [%s], tickets_released: %t)",
				actorEmail, strings.Join(updated.InternalEmailDomains, ", "), updated.TicketsReleased),
			meta)
	}

	return updated, nil
}
