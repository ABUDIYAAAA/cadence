package teams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"devclubxnst.com/cadence/pkg/audit"
	"github.com/jackc/pgx/v5"
)

// RosterManager is the cross-module ownership seam for all team mutations:
// transactional eviction (across all competitions, an event, or a single competition),
// bulk/named team formation and assignment, and team project submission updates.
type RosterManager interface {
	EvictAndAuditAllTeamsTx(ctx context.Context, tx pgx.Tx, userID string, auditor audit.Recorder) (func(), error)
	EvictAndAuditEventTeamsTx(ctx context.Context, tx pgx.Tx, eventID, userID string, auditor audit.Recorder) (func(), error)
	EvictAndAuditCompetitionTeamTx(ctx context.Context, tx pgx.Tx, compID, userID string, auditor audit.Recorder) (func(), error)
	AssignUserToNamedTeam(ctx context.Context, compID, actorID, userID, teamName string) (teamID, resolvedName string, teamCreated, newlyAssigned bool, err error)
	SetTeamSubmission(ctx context.Context, teamID, project string, links map[string]string) error
	SetTeamSubmissionTx(ctx context.Context, tx pgx.Tx, compID, teamID, project string, links map[string]string) error
}

var _ RosterManager = (*Repository)(nil)

// EvictUserFromCompetitionTeamTx removes userID from their team in compID within tx,
// transferring leadership or deleting an empty team, and returns any change for post-commit auditing.
func (r *Repository) EvictUserFromCompetitionTeamTx(ctx context.Context, tx pgx.Tx, compID, userID string) ([]EvictedTeamChange, error) {
	var tl teamLock
	var eventID string
	err := tx.QueryRow(ctx, `
		SELECT t.id, t.name, t.competition_id, c.event_id, t.leader_id, t.is_frozen, t.max_size
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		JOIN competitions c ON c.id = t.competition_id
		WHERE tm.competition_id = $1 AND tm.user_id = $2
		FOR UPDATE OF t
	`, compID, userID).Scan(&tl.ID, &tl.Name, &tl.CompetitionID, &eventID, &tl.LeaderID, &tl.Frozen, &tl.MaxSize)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find competition team for eviction: %w", err)
	}

	rem, newLeaderID, err := removeMemberTx(ctx, tx, &tl, userID)
	if err != nil {
		return nil, err
	}

	return []EvictedTeamChange{{
		TeamID:        tl.ID,
		TeamName:      tl.Name,
		CompetitionID: tl.CompetitionID,
		EventID:       eventID,
		UserID:        userID,
		Deleted:       rem == 0,
		NewLeaderID:   newLeaderID,
	}}, nil
}

// EvictAndAuditCompetitionTeamTx evicts userID from their team in compID within tx and
// returns a post-commit audit callback.
func (r *Repository) EvictAndAuditCompetitionTeamTx(ctx context.Context, tx pgx.Tx, compID, userID string, auditor audit.Recorder) (func(), error) {
	changes, err := r.EvictUserFromCompetitionTeamTx(ctx, tx, compID, userID)
	if err != nil {
		return nil, err
	}
	return func() {
		r.RecordEvictionAudits(ctx, auditor, changes)
	}, nil
}

// AssignUserToNamedTeam finds or creates teamName inside compID and assigns userID to it
// while enforcing competition team-size bounds, team freeze rules, and user-then-team lock ordering.
func (r *Repository) AssignUserToNamedTeam(
	ctx context.Context,
	compID, actorID, userID, teamName string,
) (teamID, resolvedName string, teamCreated, newlyAssigned bool, err error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		return "", "", false, false, nil
	}
	if len(teamName) < 2 || len(teamName) > 100 {
		return "", "", false, false, errors.New("team_name must be between 2 and 100 characters")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", "", false, false, fmt.Errorf("begin team assignment tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// User-first lock ordering matching CreateTeam, JoinByCode, and AddMember.
	existingTeamID, err := lockParticipant(ctx, tx, compID, userID)
	if err != nil {
		return "", "", false, false, err
	}
	if existingTeamID != nil {
		var existingName string
		if err := tx.QueryRow(ctx, `SELECT name FROM teams WHERE id = $1`, *existingTeamID).Scan(&existingName); err != nil {
			return "", "", false, false, fmt.Errorf("lookup existing team: %w", err)
		}
		if strings.EqualFold(existingName, teamName) {
			return *existingTeamID, existingName, false, false, nil
		}
		return "", "", false, false, fmt.Errorf("already in team %q", existingName)
	}

	settings, err := r.loadSettingsForComp(ctx, tx, compID)
	if err != nil {
		return "", "", false, false, fmt.Errorf("load competition settings: %w", err)
	}

	// Find or create the target team in this competition.
	var tl teamLock
	err = tx.QueryRow(ctx, `
		SELECT id, name, competition_id, leader_id, is_frozen, max_size
		FROM teams
		WHERE competition_id = $1 AND LOWER(name) = LOWER($2)
		FOR UPDATE
	`, compID, teamName).Scan(&tl.ID, &tl.Name, &tl.CompetitionID, &tl.LeaderID, &tl.Frozen, &tl.MaxSize)
	if errors.Is(err, pgx.ErrNoRows) {
		if settings.FrozenAll {
			return "", "", false, false, ErrTeamFrozen
		}
		var createErr error
		for range 5 {
			code, genErr := generateJoinCode()
			if genErr != nil {
				return "", "", false, false, fmt.Errorf("generate join code: %w", genErr)
			}
			createErr = tx.QueryRow(ctx, `
				INSERT INTO teams (competition_id, name, join_code, leader_id, created_by)
				VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid)
				RETURNING id, name
			`, compID, teamName, code, userID, actorID).Scan(&teamID, &resolvedName)
			if createErr == nil {
				teamCreated = true
				tl.ID = teamID
				tl.Name = resolvedName
				tl.LeaderID = &userID
				break
			}
			if !strings.Contains(createErr.Error(), "teams_competition_id_join_code_key") {
				return "", "", false, false, fmt.Errorf("create team %q: %w", teamName, createErr)
			}
		}
		if createErr != nil {
			return "", "", false, false, fmt.Errorf("create team %q: %w", teamName, createErr)
		}
	} else if err != nil {
		return "", "", false, false, fmt.Errorf("lookup team %q: %w", teamName, err)
	} else {
		if tl.effectiveFrozen(settings) {
			return "", "", false, false, ErrTeamFrozen
		}
		teamID = tl.ID
		resolvedName = tl.Name
		effectiveMax := tl.effectiveMax(settings)
		count, err := memberCount(ctx, tx, teamID)
		if err != nil {
			return "", "", false, false, fmt.Errorf("count team members: %w", err)
		}
		if count >= effectiveMax {
			return "", "", false, false, fmt.Errorf("team %q is full (max %d)", resolvedName, effectiveMax)
		}
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, competition_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (competition_id, user_id) DO NOTHING
	`, teamID, userID, compID)
	if err != nil {
		return "", "", false, false, fmt.Errorf("add team member: %w", err)
	}
	newlyAssigned = tag.RowsAffected() > 0

	if tl.LeaderID == nil {
		_, _ = tx.Exec(ctx, `UPDATE teams SET leader_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND leader_id IS NULL`, userID, teamID)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", false, false, fmt.Errorf("commit team assignment: %w", err)
	}

	if r.auditor != nil {
		if teamCreated {
			r.auditor.RecordAction(ctx, audit.ActionTeamCreate, audit.TargetTeam, &teamID,
				fmt.Sprintf("Created team %s via bulk/admin assignment", resolvedName),
				map[string]any{"name": resolvedName, "leader_id": userID, "competition_id": compID})
		} else if newlyAssigned {
			r.auditor.RecordAction(ctx, audit.ActionTeamAddMember, audit.TargetTeam, &teamID,
				fmt.Sprintf("Assigned user %s to team %s", userID, resolvedName),
				map[string]any{"user_id": userID, "team_id": teamID, "competition_id": compID})
		}
	}

	return teamID, resolvedName, teamCreated, newlyAssigned, nil
}

// SetTeamSubmission updates a team's project_name and submission_links on the teams table.
func (r *Repository) SetTeamSubmission(ctx context.Context, teamID, project string, links map[string]string) error {
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}
	var projArg *string
	if project != "" {
		projArg = &project
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE teams
		SET project_name = COALESCE($2, project_name),
		    submission_links = $3,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, teamID, projArg, linksJSON)
	if err != nil {
		return fmt.Errorf("update team submission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

// SetTeamSubmissionTx updates a team's project_name and non-empty submission_links inside tx,
// scoped to compID.
func (r *Repository) SetTeamSubmissionTx(ctx context.Context, tx pgx.Tx, compID, teamID, project string, links map[string]string) error {
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}
	var projArg *string
	if project != "" {
		projArg = &project
	}
	if _, err := tx.Exec(ctx, `
		UPDATE teams
		SET project_name = COALESCE($2, project_name),
		    submission_links = CASE WHEN $3::jsonb = '{}'::jsonb THEN submission_links ELSE $3::jsonb END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND competition_id = $4
	`, teamID, projArg, linksJSON, compID); err != nil {
		return fmt.Errorf("update team submission for slot %s: %w", teamID, err)
	}
	return nil
}
