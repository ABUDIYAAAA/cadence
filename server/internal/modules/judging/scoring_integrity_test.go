package judging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/competitions"
	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func newTestJudgingPool(t *testing.T, upToMigration int) (*pgxpool.Pool, string) {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping judging DB test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schemaName := fmt.Sprintf("test_judging_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping judging DB test: cannot connect to Postgres: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Skipf("skipping judging DB test: Postgres unreachable: %v", err)
	}

	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse db url: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		adminPool.Close()
		t.Fatalf("create schema-scoped pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		adminPool.Close()
	})

	applyMigrationsRange(t, ctx, pool, 1, upToMigration)
	return pool, schemaName
}

func applyMigrationsRange(t *testing.T, ctx context.Context, pool *pgxpool.Pool, minMigration, maxMigration int) {
	t.Helper()
	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var num int
		if _, err := fmt.Sscanf(name, "%06d_", &num); err == nil {
			if num < minMigration || (maxMigration > 0 && num > maxMigration) {
				continue
			}
		}
		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func TestScoringIntegrity_ForeignCriteriaAndDatabaseEnforcement(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	// Seed a judge and a team
	var judgeID, teamID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, is_active)
		VALUES ('judge.a@cadence.local', 'Judge A', true)
		RETURNING id`,
	).Scan(&judgeID); err != nil {
		t.Fatalf("seed judge: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_events (user_id, event_id, role)
		VALUES ($1, '00000000-0000-0000-0000-000000000001', 'judge')
		ON CONFLICT (event_id, user_id) DO UPDATE SET role = 'judge'`, judgeID); err != nil {
		t.Fatalf("seed user event: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO teams (name, join_code) VALUES ('Team Alpha', 'ALPHA001') RETURNING id`,
	).Scan(&teamID); err != nil {
		t.Fatalf("seed team: %v", err)
	}

	// Create Round A (2 criteria: Innovation [0..10, weight 50], Execution [2..10, weight 50])
	roundAID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name:             "Round A",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Innovation", MinScore: 0, MaxScore: 10, Weight: 50, OrderIndex: 1},
			{Name: "Execution", MinScore: 2, MaxScore: 10, Weight: 50, OrderIndex: 2},
		},
	})
	if err != nil {
		t.Fatalf("create round A: %v", err)
	}
	if err := repo.ActivateRound(ctx, roundAID); err != nil {
		t.Fatalf("activate round A: %v", err)
	}
	roundACriteria, err := repo.GetRoundCriteria(ctx, roundAID)
	if err != nil || len(roundACriteria) != 2 {
		t.Fatalf("get round A criteria: %v (len=%d)", err, len(roundACriteria))
	}
	critA1 := roundACriteria[0].ID
	critA2 := roundACriteria[1].ID

	// Create Round B (1 criterion: Finals Pitch [0..100, weight 100])
	roundBID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name:             "Round B",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Finals Pitch", MinScore: 0, MaxScore: 100, Weight: 100, OrderIndex: 1},
		},
	})
	if err != nil {
		t.Fatalf("create round B: %v", err)
	}
	roundBCriteria, err := repo.GetRoundCriteria(ctx, roundBID)
	if err != nil || len(roundBCriteria) != 1 {
		t.Fatalf("get round B criteria: %v", err)
	}
	foreignCritB := roundBCriteria[0].ID

	// Create Panel in Round A with Judge A and configure slot for Team Alpha
	panelAID, err := repo.CreatePanel(ctx, CreatePanelRequest{
		RoundID:  roundAID,
		Name:     "Panel A1",
		Room:     "Room 101",
		JudgeIDs: []string{judgeID},
	})
	if err != nil {
		t.Fatalf("create panel A: %v", err)
	}
	if err := repo.ConfigureSlots(ctx, panelAID, []SlotAssignment{
		{TeamID: teamID, SequenceOrder: 1},
	}, false); err != nil {
		t.Fatalf("configure slots: %v", err)
	}

	var slotID string
	if err := pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1`, panelAID).Scan(&slotID); err != nil {
		t.Fatalf("get slot id: %v", err)
	}
	if err := repo.StartSlot(ctx, slotID); err != nil {
		t.Fatalf("start slot: %v", err)
	}

	t.Run("1. application rejects foreign criterion from another round", func(t *testing.T) {
		err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA1, Score: 8},
				{CriterionID: foreignCritB, Score: 500},
			},
		})
		if !errors.Is(err, ErrCriterionNotInRound) {
			t.Fatalf("expected ErrCriterionNotInRound when submitting foreign criterion, got: %v", err)
		}
	})

	t.Run("2. application rejects duplicate criterion_id in a single submission", func(t *testing.T) {
		err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA1, Score: 8},
				{CriterionID: critA1, Score: 9},
			},
		})
		if !errors.Is(err, ErrDuplicateCriterion) {
			t.Fatalf("expected ErrDuplicateCriterion, got: %v", err)
		}
	})

	t.Run("3. application rejects out-of-range scores", func(t *testing.T) {
		err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA2, Score: 1}, // min is 2
			},
		})
		if !errors.Is(err, ErrScoreOutOfRange) {
			t.Fatalf("expected ErrScoreOutOfRange for score below min_score, got: %v", err)
		}
	})

	t.Run("4. database trigger and composite FKs reject direct SQL foreign criterion and out-of-bounds score", func(t *testing.T) {
		// Direct SQL insert with foreign criterion from Round B must fail at the PostgreSQL boundary
		_, err := pool.Exec(ctx, `
			INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score, is_locked)
			VALUES ($1, $2, $3, 50, false)`,
			slotID, judgeID, foreignCritB,
		)
		if err == nil {
			t.Fatal("expected PostgreSQL to reject direct SQL insert of foreign-round criterion, but it succeeded")
		}

		// Direct SQL insert with explicit mismatched round_id must also fail composite FK on slot
		_, err = pool.Exec(ctx, `
			INSERT INTO judging_scores (slot_id, round_id, judge_id, criterion_id, score, is_locked)
			VALUES ($1, $2, $3, $4, 50, false)`,
			slotID, roundBID, judgeID, foreignCritB,
		)
		if err == nil {
			t.Fatal("expected PostgreSQL composite FK fk_judging_scores_slot_round to reject mismatched round_id, but it succeeded")
		}

		// Direct SQL insert with out-of-range score for valid criterion must fail trigger check
		_, err = pool.Exec(ctx, `
			INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score, is_locked)
			VALUES ($1, $2, $3, 99, false)`,
			slotID, judgeID, critA1,
		)
		if err == nil {
			t.Fatal("expected PostgreSQL trigger to reject out-of-range score 99 > 10, but it succeeded")
		}
	})

	t.Run("5. locking requires all criteria of the slot's round to be scored", func(t *testing.T) {
		if err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA1, Score: 8},
			},
		}); err != nil {
			t.Fatalf("submit first criterion score: %v", err)
		}

		err := repo.LockJudgeScorecard(ctx, slotID, judgeID)
		if !errors.Is(err, ErrIncompleteScore) {
			t.Fatalf("expected ErrIncompleteScore when only 1 of 2 criteria scored, got: %v", err)
		}
	})

	t.Run("6. concurrent score submissions and lock enforce immutability and accurate leaderboard", func(t *testing.T) {
		if err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA1, Score: 8},
				{CriterionID: critA2, Score: 6},
			},
		}); err != nil {
			t.Fatalf("submit both criteria scores: %v", err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_ = repo.LockJudgeScorecard(ctx, slotID, judgeID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
				Scores: []CriterionScoreInput{
					{CriterionID: critA1, Score: 10},
					{CriterionID: critA2, Score: 10},
				},
			})
		}()
		close(start)
		wg.Wait()

		// Ensure scorecard is locked (if upsert ran first, lock succeeded; if lock ran first, lock succeeded)
		_ = repo.LockJudgeScorecard(ctx, slotID, judgeID)

		// Once locked, any subsequent UpsertScores must fail
		if err := svc.SubmitScores(ctx, slotID, judgeID, SubmitScoresRequest{
			Scores: []CriterionScoreInput{
				{CriterionID: critA1, Score: 2},
			},
		}); err == nil {
			t.Fatal("expected UpsertScores after lock to fail, but it succeeded")
		}

		// Direct SQL update on locked row must also be rejected by PostgreSQL trigger
		if _, err := pool.Exec(ctx, `
			UPDATE judging_scores SET score = 0 WHERE slot_id = $1 AND judge_id = $2`,
			slotID, judgeID,
		); err == nil {
			t.Fatal("expected PostgreSQL trigger to reject direct SQL score modification on locked row, but it succeeded")
		}

		allLocked, err := repo.CheckAllPanelJudgesLocked(ctx, slotID)
		if err != nil || !allLocked {
			t.Fatalf("expected CheckAllPanelJudgesLocked=true, got %v (err=%v)", allLocked, err)
		}

		lb, err := repo.GetAdminLeaderboard(ctx, roundAID)
		if err != nil {
			t.Fatalf("GetAdminLeaderboard: %v", err)
		}
		if len(lb.Leaderboard) != 1 {
			t.Fatalf("expected 1 leaderboard entry, got %d", len(lb.Leaderboard))
		}
		entry := lb.Leaderboard[0]
		if entry.TotalWeightedScore <= 0 || entry.TotalWeightedScore > lb.MaxPossibleScore {
			t.Fatalf("unexpected total_weighted_score %.2f (max %.2f)", entry.TotalWeightedScore, lb.MaxPossibleScore)
		}
	})
}

func TestMultiTenantCompetitionIsolationAndSettings(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	baseCtx := context.Background()

	var eventAID, eventBID, compA1ID, compA2ID, compB1ID string
	if err := pool.QueryRow(baseCtx, `INSERT INTO events (slug, name) VALUES ('event-a', 'Event A') RETURNING id`).Scan(&eventAID); err != nil {
		t.Fatalf("create event A: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO events (slug, name) VALUES ('event-b', 'Event B') RETURNING id`).Scan(&eventBID); err != nil {
		t.Fatalf("create event B: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, 'comp-a1', 'Comp A1') RETURNING id`, eventAID).Scan(&compA1ID); err != nil {
		t.Fatalf("create comp A1: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, 'comp-a2', 'Comp A2') RETURNING id`, eventAID).Scan(&compA2ID); err != nil {
		t.Fatalf("create comp A2: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, 'comp-b1', 'Comp B1') RETURNING id`, eventBID).Scan(&compB1ID); err != nil {
		t.Fatalf("create comp B1: %v", err)
	}

	var judgeID, teamA1ID, teamA2ID string
	if err := pool.QueryRow(baseCtx, `INSERT INTO users (email, name, is_active) VALUES ('judge.multi@cadence.local', 'Judge Multi', true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatalf("create judge: %v", err)
	}
	for _, evID := range []string{eventAID, eventBID} {
		if _, err := pool.Exec(baseCtx, `INSERT INTO user_events (user_id, event_id, role) VALUES ($1, $2, 'judge')`, judgeID, evID); err != nil {
			t.Fatalf("assign judge to event: %v", err)
		}
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO teams (competition_id, name, join_code) VALUES ($1, 'Team A1', 'TEAMA1') RETURNING id`, compA1ID).Scan(&teamA1ID); err != nil {
		t.Fatalf("create team A1: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO teams (competition_id, name, join_code) VALUES ($1, 'Team A2', 'TEAMA2') RETURNING id`, compA2ID).Scan(&teamA2ID); err != nil {
		t.Fatalf("create team A2: %v", err)
	}

	ctxCompA1 := competitions.WithCompetitionContext(
		events.WithEventContext(baseCtx, &events.Context{EventID: eventAID, Slug: "event-a", Name: "Event A", UserID: judgeID, Role: "owner"}),
		&competitions.Context{CompetitionID: compA1ID, EventID: eventAID, Slug: "comp-a1", Name: "Comp A1", UserID: judgeID, Role: "owner"},
	)
	ctxCompA2 := competitions.WithCompetitionContext(
		events.WithEventContext(baseCtx, &events.Context{EventID: eventAID, Slug: "event-a", Name: "Event A", UserID: judgeID, Role: "owner"}),
		&competitions.Context{CompetitionID: compA2ID, EventID: eventAID, Slug: "comp-a2", Name: "Comp A2", UserID: judgeID, Role: "owner"},
	)
	ctxCompB1 := competitions.WithCompetitionContext(
		events.WithEventContext(baseCtx, &events.Context{EventID: eventBID, Slug: "event-b", Name: "Event B", UserID: judgeID, Role: "owner"}),
		&competitions.Context{CompetitionID: compB1ID, EventID: eventBID, Slug: "comp-b1", Name: "Comp B1", UserID: judgeID, Role: "owner"},
	)

	// Create and activate round in Comp A1
	roundA1ID, err := repo.CreateRound(ctxCompA1, CreateRoundRequest{
		Name:             "Round A1",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Impact", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1},
		},
	})
	if err != nil {
		t.Fatalf("create round A1: %v", err)
	}
	panelA1ID, err := repo.CreatePanel(ctxCompA1, CreatePanelRequest{
		RoundID:  roundA1ID,
		Name:     "Panel A1",
		Room:     "Room A1",
		JudgeIDs: []string{judgeID},
	})
	if err != nil {
		t.Fatalf("create panel A1: %v", err)
	}
	if err := repo.ConfigureSlots(ctxCompA1, panelA1ID, []SlotAssignment{
		{TeamID: teamA1ID, SequenceOrder: 1},
	}, false); err != nil {
		t.Fatalf("configure slots A1: %v", err)
	}
	if err := repo.ActivateRound(ctxCompA1, roundA1ID); err != nil {
		t.Fatalf("activate round A1: %v", err)
	}
	var slotA1ID string
	if err := pool.QueryRow(baseCtx, `SELECT id FROM judging_slots WHERE panel_id = $1`, panelA1ID).Scan(&slotA1ID); err != nil {
		t.Fatalf("get slot A1: %v", err)
	}

	t.Run("cross-competition and cross-event GetJudgePanels and GetVolunteerSchedule do not leak active rounds", func(t *testing.T) {
		panelsInA2, err := repo.GetJudgePanels(ctxCompA2, judgeID)
		if err != nil {
			t.Fatalf("GetJudgePanels in A2: %v", err)
		}
		if len(panelsInA2) != 0 {
			t.Fatalf("expected 0 judge panels in Comp A2, got %d", len(panelsInA2))
		}
		panelsInB1, err := repo.GetJudgePanels(ctxCompB1, judgeID)
		if err != nil {
			t.Fatalf("GetJudgePanels in B1: %v", err)
		}
		if len(panelsInB1) != 0 {
			t.Fatalf("expected 0 judge panels in Event B Comp B1, got %d", len(panelsInB1))
		}
		schedInB1, err := repo.GetVolunteerSchedule(ctxCompB1, nil)
		if err != nil {
			t.Fatalf("GetVolunteerSchedule in B1: %v", err)
		}
		if len(schedInB1) != 0 {
			t.Fatalf("expected 0 volunteer schedule panels in Event B Comp B1, got %d", len(schedInB1))
		}
	})

	t.Run("cross-competition access to Round, Panel, Slot returns 404 NotFound errors", func(t *testing.T) {
		if _, _, _, _, _, err := repo.GetRound(ctxCompA2, roundA1ID); !errors.Is(err, ErrRoundNotFound) {
			t.Fatalf("expected ErrRoundNotFound for cross-competition GetRound, got %v", err)
		}
		if err := repo.EndRound(ctxCompA2, roundA1ID, true); !errors.Is(err, ErrRoundNotFound) {
			t.Fatalf("expected ErrRoundNotFound for cross-competition EndRound, got %v", err)
		}
		if _, err := repo.GetAdminLeaderboard(ctxCompB1, roundA1ID); !errors.Is(err, ErrRoundNotFound) {
			t.Fatalf("expected ErrRoundNotFound for cross-event GetAdminLeaderboard, got %v", err)
		}
		if err := repo.ConfigureSlots(ctxCompA2, panelA1ID, []SlotAssignment{{TeamID: teamA2ID, SequenceOrder: 1}}, false); !errors.Is(err, ErrPanelNotFound) {
			t.Fatalf("expected ErrPanelNotFound for cross-competition ConfigureSlots, got %v", err)
		}
		if err := repo.ConfigureSlots(ctxCompA1, panelA1ID, []SlotAssignment{{TeamID: teamA2ID, SequenceOrder: 2}}, true); !errors.Is(err, ErrUnknownTeam) {
			t.Fatalf("expected ErrUnknownTeam when slotting a team from Comp A2 into Comp A1, got %v", err)
		}
		if err := repo.StartSlot(ctxCompA2, slotA1ID); !errors.Is(err, ErrSlotNotFound) {
			t.Fatalf("expected ErrSlotNotFound for cross-competition StartSlot, got %v", err)
		}
	})

	t.Run("event owner and dev can update slot links without 403 Forbidden", func(t *testing.T) {
		if err := svc.UpdateSlotLinks(ctxCompA1, slotA1ID, judgeID, "owner@cadence.local", "owner", map[string]string{"github": "https://github.com/owner/repo"}, "127.0.0.1", "test"); err != nil {
			t.Fatalf("expected owner to update slot links, got: %v", err)
		}
		if err := svc.UpdateSlotLinks(ctxCompA1, slotA1ID, judgeID, "dev@cadence.local", "dev", map[string]string{"github": "https://github.com/dev/repo"}, "127.0.0.1", "test"); err != nil {
			t.Fatalf("expected dev to update slot links, got: %v", err)
		}
	})

	t.Run("submission_gate_session is isolated per competition within the same event", func(t *testing.T) {
		var sessID string
		if err := pool.QueryRow(baseCtx, `INSERT INTO scan_sessions (event_id, name, kind) VALUES ($1, 'Gate A1', 'attendance') RETURNING id`, eventAID).Scan(&sessID); err != nil {
			t.Fatalf("create scan session: %v", err)
		}
		if _, err := repo.SetSubmissionGate(ctxCompA1, "", &sessID, judgeID); err != nil {
			t.Fatalf("set submission gate on Comp A1: %v", err)
		}
		gateA1, err := repo.GetSubmissionGate(ctxCompA1, "")
		if err != nil || gateA1.SessionID == nil || *gateA1.SessionID != sessID {
			t.Fatalf("expected Comp A1 gate=%s, got %+v (err=%v)", sessID, gateA1, err)
		}
		gateA2, err := repo.GetSubmissionGate(ctxCompA2, "")
		if err != nil {
			t.Fatalf("get submission gate on Comp A2: %v", err)
		}
		if gateA2.SessionID != nil {
			t.Fatalf("expected Comp A2 submission gate to remain unset (nil), got %v", *gateA2.SessionID)
		}
	})
}

func TestSubmissionSingleSourceOfTruthAndGateConsistency(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	baseCtx := context.Background()

	// 1. Verify redundant judging_slots.submission_links column is dropped from the schema
	var colCount int
	if err := pool.QueryRow(baseCtx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'judging_slots' AND column_name = 'submission_links'
	`).Scan(&colCount); err != nil {
		t.Fatalf("check judging_slots.submission_links column: %v", err)
	}
	if colCount != 0 {
		t.Fatalf("expected judging_slots.submission_links column to be dropped (0 rows), got %d", colCount)
	}

	var eventID, compID, judgeID, leaderID, teamID string
	if err := pool.QueryRow(baseCtx, `INSERT INTO events (slug, name) VALUES ('ev-sub', 'Event Sub') RETURNING id`).Scan(&eventID); err != nil {
		t.Fatalf("create event: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, 'comp-sub', 'Comp Sub') RETURNING id`, eventID).Scan(&compID); err != nil {
		t.Fatalf("create comp: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO users (email, name, is_active) VALUES ('judge.sub@cadence.local', 'Judge Sub', true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatalf("create judge: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO users (email, name, is_active) VALUES ('lead.sub@cadence.local', 'Lead Sub', true) RETURNING id`).Scan(&leaderID); err != nil {
		t.Fatalf("create leader: %v", err)
	}
	if _, err := pool.Exec(baseCtx, `INSERT INTO user_events (user_id, event_id, role) VALUES ($1, $2, 'judge'), ($3, $2, 'participant')`, judgeID, eventID, leaderID); err != nil {
		t.Fatalf("assign event roles: %v", err)
	}
	if err := pool.QueryRow(baseCtx, `INSERT INTO teams (competition_id, name, join_code, leader_id) VALUES ($1, 'Team Sub', 'SUB001', $2) RETURNING id`, compID, leaderID).Scan(&teamID); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := pool.Exec(baseCtx, `INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3)`, teamID, leaderID, compID); err != nil {
		t.Fatalf("insert team member: %v", err)
	}

	ctxComp := competitions.WithCompetitionContext(
		events.WithEventContext(baseCtx, &events.Context{EventID: eventID, Slug: "ev-sub", Name: "Event Sub", UserID: leaderID, Role: "participant"}),
		&competitions.Context{CompetitionID: compID, EventID: eventID, Slug: "comp-sub", Name: "Comp Sub", UserID: leaderID, Role: "participant"},
	)

	roundID, err := repo.CreateRound(ctxComp, CreateRoundRequest{
		Name:             "Round Sub",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Code", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1},
		},
	})
	if err != nil {
		t.Fatalf("create round: %v", err)
	}
	panelID, err := repo.CreatePanel(ctxComp, CreatePanelRequest{
		RoundID:  roundID,
		Name:     "Panel Sub",
		Room:     "Room 201",
		JudgeIDs: []string{judgeID},
	})
	if err != nil {
		t.Fatalf("create panel: %v", err)
	}
	if err := repo.ConfigureSlots(ctxComp, panelID, []SlotAssignment{
		{TeamID: teamID, SequenceOrder: 1, SubmissionLinks: map[string]string{"github": "https://github.com/cadence/seeded"}},
	}, false); err != nil {
		t.Fatalf("configure slots: %v", err)
	}
	if err := repo.ActivateRound(ctxComp, roundID); err != nil {
		t.Fatalf("activate round: %v", err)
	}
	var slotID string
	if err := pool.QueryRow(baseCtx, `SELECT id FROM judging_slots WHERE panel_id = $1`, panelID).Scan(&slotID); err != nil {
		t.Fatalf("get slot id: %v", err)
	}

	// 2. When no attendance gate session is configured on the active round (optional gate),
	// BOTH GetTeamStatusForUser and GetMySubmission must report CanSubmit == true immediately!
	statusResp, err := svc.GetTeamStatusForUser(ctxComp, leaderID)
	if err != nil {
		t.Fatalf("GetTeamStatusForUser: %v", err)
	}
	mySub, err := svc.GetMySubmission(ctxComp, leaderID)
	if err != nil {
		t.Fatalf("GetMySubmission: %v", err)
	}
	if !statusResp.CanSubmit || !mySub.CanSubmit {
		t.Fatalf("expected both statusResp.CanSubmit and mySub.CanSubmit to be true when no gate session is set; got status=%v (%q), mySub=%v (%q)",
			statusResp.CanSubmit, statusResp.SubmitBlockedReason, mySub.CanSubmit, mySub.SubmitBlockedReason)
	}

	// 2b. Once an optional attendance gate IS set on the active round, an unscanned team is blocked
	// consistently across both GetTeamStatusForUser and GetMySubmission.
	var gateSessID string
	if err := pool.QueryRow(baseCtx, `INSERT INTO scan_sessions (event_id, name, kind) VALUES ($1, 'Night 1 Gate', 'attendance') RETURNING id`, eventID).Scan(&gateSessID); err != nil {
		t.Fatalf("create gate scan session: %v", err)
	}
	if _, err := repo.SetSubmissionGate(ctxComp, roundID, &gateSessID, judgeID); err != nil {
		t.Fatalf("set round submission gate: %v", err)
	}
	statusBlocked, err := svc.GetTeamStatusForUser(ctxComp, leaderID)
	if err != nil {
		t.Fatalf("GetTeamStatusForUser (gated): %v", err)
	}
	mySubBlocked, err := svc.GetMySubmission(ctxComp, leaderID)
	if err != nil {
		t.Fatalf("GetMySubmission (gated): %v", err)
	}
	if statusBlocked.CanSubmit || mySubBlocked.CanSubmit {
		t.Fatalf("expected both CanSubmit=false when round gate is set and team not scanned; got status=%v, mySub=%v", statusBlocked.CanSubmit, mySubBlocked.CanSubmit)
	}
	if statusBlocked.SubmitBlockedReason == "" || statusBlocked.SubmitBlockedReason != mySubBlocked.SubmitBlockedReason {
		t.Fatalf("expected identical non-empty SubmitBlockedReason, got status=%q vs mySub=%q",
			statusBlocked.SubmitBlockedReason, mySubBlocked.SubmitBlockedReason)
	}
	// Clearing the gate re-opens submissions immediately.
	if _, err := repo.SetSubmissionGate(ctxComp, roundID, nil, judgeID); err != nil {
		t.Fatalf("clear round submission gate: %v", err)
	}

	// 3. Updating slot links via UpdateSlotLinks (as admin) writes to teams and is immediately reflected in:
	//    - GetMySubmission
	//    - ListSubmissions (CSV export)
	//    - GetJudgeSlotDetails
	//    - GetVolunteerSchedule
	//    - GetTeamStatusForUser
	if err := svc.UpdateSlotLinks(ctxComp, slotID, judgeID, "admin@cadence.local", "admin", map[string]string{
		"project_name": "Unified Project",
		"github":       "https://github.com/cadence/unified-repo",
		"demo":         "https://demo.cadence.local",
		"arbitrary":    "https://ignored.invalid",
	}, "127.0.0.1", "test"); err != nil {
		t.Fatalf("UpdateSlotLinks: %v", err)
	}

	mySubAfter, err := svc.GetMySubmission(ctxComp, leaderID)
	if err != nil {
		t.Fatalf("GetMySubmission after UpdateSlotLinks: %v", err)
	}
	if mySubAfter.ProjectName != "Unified Project" || mySubAfter.Links["github"] != "https://github.com/cadence/unified-repo" || mySubAfter.Links["demo"] != "https://demo.cadence.local" {
		t.Fatalf("expected GetMySubmission to reflect UpdateSlotLinks on teams table, got %+v", mySubAfter)
	}
	if _, hasArbitrary := mySubAfter.Links["arbitrary"]; hasArbitrary {
		t.Fatalf("expected UpdateSlotLinks to filter out disallowed link keys, got %+v", mySubAfter.Links)
	}

	exportResp, err := svc.ListSubmissions(ctxComp)
	if err != nil {
		t.Fatalf("ListSubmissions: %v", err)
	}
	if len(exportResp.Submissions) != 1 || exportResp.Submissions[0].ProjectName != "Unified Project" || exportResp.Submissions[0].Repository != "https://github.com/cadence/unified-repo" {
		t.Fatalf("expected ListSubmissions CSV export to reflect UpdateSlotLinks, got %+v", exportResp.Submissions)
	}

	slotDetails, err := svc.GetJudgeSlotDetails(ctxComp, slotID, judgeID)
	if err != nil {
		t.Fatalf("GetJudgeSlotDetails: %v", err)
	}
	if slotDetails.SubmissionLinks["github"] != "https://github.com/cadence/unified-repo" || slotDetails.SubmissionLinks["project_name"] != "Unified Project" {
		t.Fatalf("expected GetJudgeSlotDetails to join submission_links from teams, got %+v", slotDetails.SubmissionLinks)
	}

	volSched, err := svc.GetVolunteerSchedule(ctxComp, nil)
	if err != nil || len(volSched) != 1 || len(volSched[0].QueuedTeams) != 1 {
		t.Fatalf("GetVolunteerSchedule: err=%v sched=%+v", err, volSched)
	}
	if volSched[0].QueuedTeams[0].SubmissionLinks["github"] != "https://github.com/cadence/unified-repo" || volSched[0].QueuedTeams[0].SubmissionLinks["project_name"] != "Unified Project" {
		t.Fatalf("expected GetVolunteerSchedule to join submission_links from teams, got %+v", volSched[0].QueuedTeams[0].SubmissionLinks)
	}
}

