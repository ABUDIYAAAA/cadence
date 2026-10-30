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
	pool, _ := newTestJudgingPool(t, 14)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	// Seed a judge and a team
	var judgeID, teamID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.a@cadence.local', 'Judge A', 'judge', true, true)
		RETURNING id`,
	).Scan(&judgeID); err != nil {
		t.Fatalf("seed judge: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO teams (name) VALUES ('Team Alpha') RETURNING id`,
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

func TestScoringIntegrity_LegacyDataMigrationAndIsolation(t *testing.T) {
	// Apply migrations up to 000013 (before 000014 database constraints) to simulate legacy corrupted data
	pool, _ := newTestJudgingPool(t, 13)
	repo := NewRepository(pool)
	ctx := context.Background()

	var judgeID, teamID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.legacy@cadence.local', 'Judge Legacy', 'judge', true, true)
		RETURNING id`,
	).Scan(&judgeID); err != nil {
		t.Fatalf("seed judge: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO teams (name) VALUES ('Team Legacy') RETURNING id`,
	).Scan(&teamID); err != nil {
		t.Fatalf("seed team: %v", err)
	}

	roundAID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name:             "Legacy Round A",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Crit A1", MinScore: 0, MaxScore: 10, Weight: 50, OrderIndex: 1},
			{Name: "Crit A2", MinScore: 0, MaxScore: 10, Weight: 50, OrderIndex: 2},
		},
	})
	if err != nil {
		t.Fatalf("create round A: %v", err)
	}
	if err := repo.ActivateRound(ctx, roundAID); err != nil {
		t.Fatalf("activate round A: %v", err)
	}
	roundACrit, _ := repo.GetRoundCriteria(ctx, roundAID)

	roundBID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name:             "Legacy Round B",
		BufferSize:       2,
		SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Foreign Crit B1", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1},
		},
	})
	if err != nil {
		t.Fatalf("create round B: %v", err)
	}
	roundBCrit, _ := repo.GetRoundCriteria(ctx, roundBID)

	panelAID, err := repo.CreatePanel(ctx, CreatePanelRequest{
		RoundID:  roundAID,
		Name:     "Legacy Panel A",
		Room:     "Room 202",
		JudgeIDs: []string{judgeID},
	})
	if err != nil {
		t.Fatalf("create panel: %v", err)
	}
	if err := repo.ConfigureSlots(ctx, panelAID, []SlotAssignment{
		{TeamID: teamID, SequenceOrder: 1},
	}, false); err != nil {
		t.Fatalf("configure slots: %v", err)
	}
	var slotID string
	if err := pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1`, panelAID).Scan(&slotID); err != nil {
		t.Fatalf("get slot: %v", err)
	}
	if err := repo.StartSlot(ctx, slotID); err != nil {
		t.Fatalf("start slot: %v", err)
	}

	// Simulate pre-migration legacy state: 1 valid Round A criterion + 1 foreign Round B criterion with score 500
	if _, err := pool.Exec(ctx, `
		INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score, is_locked)
		VALUES ($1, $2, $3, 5, true), ($1, $2, $4, 500, true)`,
		slotID, judgeID, roundACrit[0].ID, roundBCrit[0].ID,
	); err != nil {
		t.Fatalf("insert legacy rows: %v", err)
	}

	// Even before migration 000014 runs, application queries must ignore the foreign criterion:
	// 1. CheckAllPanelJudgesLocked must be false (only 1 of 2 Round A criteria is locked)
	locked, err := repo.CheckAllPanelJudgesLocked(ctx, slotID)
	if err != nil {
		t.Fatalf("CheckAllPanelJudgesLocked: %v", err)
	}
	if locked {
		t.Fatal("expected CheckAllPanelJudgesLocked=false when 2nd criterion is from a foreign round")
	}

	// 2. Leaderboard must not include the foreign criterion's score (500)
	lb, err := repo.GetAdminLeaderboard(ctx, roundAID)
	if err != nil {
		t.Fatalf("GetAdminLeaderboard: %v", err)
	}
	if len(lb.Leaderboard) != 1 {
		t.Fatalf("expected 1 leaderboard entry, got %d", len(lb.Leaderboard))
	}
	// Only Crit A1 (score 5 / 10 * weight 50 = 25) should be counted, not Foreign Crit B1
	if lb.Leaderboard[0].TotalWeightedScore != 25 {
		t.Fatalf("expected TotalWeightedScore=25.00 (ignoring foreign criterion), got %.2f", lb.Leaderboard[0].TotalWeightedScore)
	}

	// 3. Now apply migration 000014 and verify the foreign row was purged and constraints are active
	applyMigrationsRange(t, ctx, pool, 14, 14)

	var remainingForeign int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM judging_scores WHERE slot_id = $1 AND criterion_id = $2`,
		slotID, roundBCrit[0].ID,
	).Scan(&remainingForeign); err != nil {
		t.Fatalf("count remaining foreign rows: %v", err)
	}
	if remainingForeign != 0 {
		t.Fatalf("expected migration 000014 to purge foreign score row, found %d", remainingForeign)
	}
}
