package competitions

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"devclubxnst.com/cadence/pkg/audit"
	"github.com/redis/go-redis/v9"
)

var (
	slugRegex      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	ErrInvalidSlug = errors.New("slug must contain only lowercase letters, numbers, and single hyphens")
)

type Service struct {
	repo    *Repository
	rdb     *redis.Client
	auditor audit.Recorder
}

func NewService(repo *Repository, rdb *redis.Client, auditor audit.Recorder) *Service {
	return &Service{
		repo:    repo,
		rdb:     rdb,
		auditor: auditor,
	}
}

// CreateCompetition creates a competition inside an event. Accessible to event owners, admins, and devs.
func (s *Service) CreateCompetition(ctx context.Context, eventID string, req CreateCompetitionRequest, actorID string) (*Competition, error) {
	if !slugRegex.MatchString(req.Slug) {
		return nil, ErrInvalidSlug
	}

	comp, err := s.repo.CreateCompetition(ctx, eventID, req.Slug, req.Name, req.Description)
	if err != nil {
		return nil, err
	}

	s.recordAudit(ctx, audit.ActionCompetitionCreate, eventID, comp.ID, fmt.Sprintf("Created competition %s (%s) in event %s", comp.Name, comp.Slug, eventID), &actorID, map[string]any{
		"event_id": eventID,
		"slug":     comp.Slug,
		"name":     comp.Name,
	})

	return comp, nil
}

// ListCompetitionsByEvent lists all competitions belonging to an event.
func (s *Service) ListCompetitionsByEvent(ctx context.Context, eventID string) ([]Competition, error) {
	return s.repo.ListCompetitionsByEvent(ctx, eventID)
}

// GetCompetitionBySlug retrieves competition metadata within an event.
func (s *Service) GetCompetitionBySlug(ctx context.Context, eventID, slug string) (*Competition, error) {
	return s.repo.GetCompetitionBySlug(ctx, eventID, slug)
}

// UpdateCompetition updates competition slug, name, and description and purges Redis cache.
func (s *Service) UpdateCompetition(ctx context.Context, eventID, compID, oldSlug string, req UpdateCompetitionRequest, actorID string) (*Competition, error) {
	if req.Slug != nil && !slugRegex.MatchString(*req.Slug) {
		return nil, ErrInvalidSlug
	}

	comp, err := s.repo.UpdateCompetition(ctx, compID, req.Slug, req.Name, req.Description)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("event:%s:comp:slug:%s", eventID, oldSlug)).Err()
		_ = s.rdb.Del(ctx, fmt.Sprintf("event:%s:comp:slug:%s", eventID, comp.Slug)).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionUpdate, eventID, comp.ID, fmt.Sprintf("Updated competition %s", comp.Slug), &actorID, map[string]any{
		"slug": comp.Slug,
		"name": comp.Name,
	})

	return comp, nil
}

// DeleteCompetition deletes a competition from an event and invalidates Redis cache.
func (s *Service) DeleteCompetition(ctx context.Context, eventID, compID, actorID string) error {
	comp, err := s.repo.DeleteCompetition(ctx, compID)
	if err != nil {
		return err
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, fmt.Sprintf("event:%s:comp:slug:%s", comp.EventID, comp.Slug)).Err()
	}

	s.recordAudit(ctx, audit.ActionCompetitionDelete, eventID, comp.ID, fmt.Sprintf("Deleted competition %s (%s)", comp.Name, comp.Slug), &actorID, map[string]any{
		"slug": comp.Slug,
		"name": comp.Name,
	})

	return nil
}

func (s *Service) recordAudit(ctx context.Context, action, eventID, compID, description string, actorID *string, meta map[string]any) {
	if s.auditor == nil {
		return
	}
	s.auditor.Record(ctx, audit.Event{
		EventID:       &eventID,
		CompetitionID: &compID,
		ActorID:       actorID,
		Action:        action,
		TargetType:    audit.TargetCompetition,
		TargetID:      &compID,
		Description:   description,
		Metadata:      meta,
	})
}
