package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/competitions"
	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/internal/modules/users"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func newTestDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@127.0.0.1:5434/lms?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping db test, db unreachable: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

// createTestUser creates an active user with a given email and role in the database.
func createTestUser(t *testing.T, pool *pgxpool.Pool, email, name, role string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id
	`, email, name, role).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

// createTestEventAndCompetition creates an event, links an owner in user_events, and creates a competition inside the event.
func createTestEventAndCompetition(t *testing.T, pool *pgxpool.Pool, eventSlug, compSlug, name, ownerID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	var eventID string
	err := pool.QueryRow(ctx, `
		INSERT INTO events (slug, name)
		VALUES ($1, $2)
		RETURNING id
	`, eventSlug, name+" Event").Scan(&eventID)
	if err != nil {
		t.Fatalf("failed to create test event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM events WHERE id = $1`, eventID)
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, eventID, ownerID)
	if err != nil {
		t.Fatalf("failed to link owner to test event: %v", err)
	}

	var compID string
	err = pool.QueryRow(ctx, `
		INSERT INTO competitions (event_id, slug, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, eventID, compSlug, name).Scan(&compID)
	if err != nil {
		t.Fatalf("failed to create test competition: %v", err)
	}

	return eventID, compID
}

// linkEventMember adds a user to an event with a given role.
func linkEventMember(t *testing.T, pool *pgxpool.Pool, eventID, userID, role string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, $3::event_role)
		ON CONFLICT (event_id, user_id) DO UPDATE SET role = EXCLUDED.role, is_banned = FALSE
	`, eventID, userID, role)
	if err != nil {
		t.Fatalf("failed to link member to event: %v", err)
	}
}

func TestTeamingWorkflow(t *testing.T) {
	pool := newTestDBPool(t)

	ctx := context.Background()
	ts := time.Now().UnixNano()

	// 1. Setup users, event, & competition
	ownerID := createTestUser(t, pool, fmt.Sprintf("owner_%d@example.com", ts), "Owner", "user")
	eventSlug := fmt.Sprintf("event-teaming-%d", ts)
	compSlug := fmt.Sprintf("comp-teaming-%d", ts)
	eventID, compID := createTestEventAndCompetition(t, pool, eventSlug, compSlug, "Teaming Test Comp", ownerID)

	// Create a second competition in the SAME event to verify multi-competition team membership
	var comp2ID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO competitions (event_id, slug, name)
		VALUES ($1, $2, 'Second Competition')
		RETURNING id
	`, eventID, fmt.Sprintf("comp2-teaming-%d", ts)).Scan(&comp2ID); err != nil {
		t.Fatalf("failed to create second competition in event: %v", err)
	}

	p1ID := createTestUser(t, pool, fmt.Sprintf("p1_%d@example.com", ts), "Alice (Leader)", "user")
	p2ID := createTestUser(t, pool, fmt.Sprintf("p2_%d@example.com", ts), "Bob (Member 1)", "user")
	p3ID := createTestUser(t, pool, fmt.Sprintf("p3_%d@example.com", ts), "Charlie (Member 2)", "user")

	linkEventMember(t, pool, eventID, p1ID, "participant")
	linkEventMember(t, pool, eventID, p2ID, "participant")
	linkEventMember(t, pool, eventID, p3ID, "participant")

	teamsRepo := NewRepository(pool, 4)
	eventsRepo := events.NewRepository(pool)
	usersRepo := users.NewRepository(pool)

	// Step A: Participant 1 creates a team in Competition 1
	teamName := fmt.Sprintf("Team Rockets %d", ts)
	teamID, err := teamsRepo.CreateTeam(ctx, compID, p1ID, teamName)
	if err != nil {
		t.Fatalf("failed to create team: %v", err)
	}

	createdTeam, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get created team: %v", err)
	}
	if createdTeam.Name != teamName {
		t.Fatalf("expected team name %s, got %s", teamName, createdTeam.Name)
	}
	if createdTeam.JoinCode == "" {
		t.Fatalf("expected non-empty join code")
	}
	if createdTeam.Leader == nil || createdTeam.Leader.ID != p1ID {
		t.Fatalf("expected creator to be leader")
	}
	if len(createdTeam.Members) != 1 || createdTeam.Members[0].ID != p1ID {
		t.Fatalf("expected creator to be sole member initially")
	}

	// Step A2: Participant 1 can also create/join a team in Competition 2 within the SAME event!
	comp2TeamID, err := teamsRepo.CreateTeam(ctx, comp2ID, p1ID, fmt.Sprintf("Comp2 Team %d", ts))
	if err != nil {
		t.Fatalf("expected participant to be able to join a team in a second competition of the same event, got: %v", err)
	}
	_, _ = teamsRepo.LeaveTeam(ctx, comp2ID, p1ID)
	_ = comp2TeamID

	// Step B: Cannot create another team in the same competition while already in a team
	_, err = teamsRepo.CreateTeam(ctx, compID, p1ID, fmt.Sprintf("Another Team %d", ts))
	if err != ErrAlreadyInTeam {
		t.Fatalf("expected ErrAlreadyInTeam, got %v", err)
	}

	// Step C: Participant 2 joins with invalid code -> ErrInvalidCode
	_, err = teamsRepo.JoinByCode(ctx, compID, p2ID, "INVALID")
	if err != ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}

	// Step D: Participant 2 joins with valid code
	_, err = teamsRepo.JoinByCode(ctx, compID, p2ID, createdTeam.JoinCode)
	if err != nil {
		t.Fatalf("failed to join team by code: %v", err)
	}
	joinedTeam, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get team after p2 joined: %v", err)
	}
	if len(joinedTeam.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(joinedTeam.Members))
	}
	if joinedTeam.Leader == nil || joinedTeam.Leader.ID != p1ID {
		t.Fatalf("expected Alice to remain leader, got %v", joinedTeam.Leader)
	}

	// Step E: Participant 3 joins
	_, err = teamsRepo.JoinByCode(ctx, compID, p3ID, createdTeam.JoinCode)
	if err != nil {
		t.Fatalf("failed for p3 to join: %v", err)
	}
	joinedTeam3, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get team after p3 joined: %v", err)
	}
	if len(joinedTeam3.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(joinedTeam3.Members))
	}

	// Step F: Test Max Size enforcement
	_, err = pool.Exec(ctx, `UPDATE teams SET max_size = 3 WHERE id = $1`, teamID)
	if err != nil {
		t.Fatalf("failed to set max_size: %v", err)
	}
	p4ID := createTestUser(t, pool, fmt.Sprintf("p4_%d@example.com", ts), "David", "user")
	linkEventMember(t, pool, eventID, p4ID, "participant")
	_, err = teamsRepo.JoinByCode(ctx, compID, p4ID, createdTeam.JoinCode)
	if err != ErrTeamFull {
		t.Fatalf("expected ErrTeamFull, got %v", err)
	}

	// Step G: Test rename and regenerate code by non-leader should fail
	_, err = teamsRepo.RenameOwnTeam(ctx, compID, p2ID, "Hacked Team Name")
	if err != ErrNotLeader {
		t.Fatalf("expected ErrNotLeader on rename, got %v", err)
	}
	_, err = teamsRepo.RegenerateOwnCode(ctx, compID, p2ID)
	if err != ErrNotLeader {
		t.Fatalf("expected ErrNotLeader on regenerate code, got %v", err)
	}

	// Leader renames and regenerates code
	newTeamName := fmt.Sprintf("Renamed Rockets %d", ts)
	_, err = teamsRepo.RenameOwnTeam(ctx, compID, p1ID, newTeamName)
	if err != nil {
		t.Fatalf("failed to rename team: %v", err)
	}
	newCode, err := teamsRepo.RegenerateOwnCode(ctx, compID, p1ID)
	if err != nil {
		t.Fatalf("failed to regenerate join code: %v", err)
	}
	if newCode == createdTeam.JoinCode {
		t.Fatalf("expected new join code to be different")
	}

	// Old code should fail now
	_, err = teamsRepo.JoinByCode(ctx, compID, p4ID, createdTeam.JoinCode)
	if err != ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode for old code, got %v", err)
	}

	// Step H: Explicit Leadership Transfer
	_, err = teamsRepo.TransferLeadership(ctx, compID, p1ID, p3ID)
	if err != nil {
		t.Fatalf("failed to transfer leadership to Charlie: %v", err)
	}
	teamAfterTransfer, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get team: %v", err)
	}
	if teamAfterTransfer.Leader == nil || teamAfterTransfer.Leader.ID != p3ID {
		t.Fatalf("expected Charlie (p3) to be new leader, got %v", teamAfterTransfer.Leader)
	}

	// Step I: Charlie kicks David (not in team) -> ErrMemberNotFound
	_, err = teamsRepo.KickMember(ctx, compID, p3ID, p4ID)
	if err != ErrMemberNotFound {
		t.Fatalf("expected ErrMemberNotFound, got %v", err)
	}

	// Charlie kicks Bob (p2)
	_, err = teamsRepo.KickMember(ctx, compID, p3ID, p2ID)
	if err != nil {
		t.Fatalf("failed to kick Bob: %v", err)
	}
	teamAfterKick, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get team: %v", err)
	}
	if len(teamAfterKick.Members) != 2 {
		t.Fatalf("expected 2 members after kick, got %d", len(teamAfterKick.Members))
	}

	// Step J: Automatic leadership succession when Leader (Charlie / p3) leaves
	_, err = teamsRepo.LeaveTeam(ctx, compID, p3ID)
	if err != nil {
		t.Fatalf("failed for leader Charlie to leave: %v", err)
	}
	teamAfterLeave, err := teamsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		t.Fatalf("failed to get team: %v", err)
	}
	if len(teamAfterLeave.Members) != 1 {
		t.Fatalf("expected 1 member remaining, got %d", len(teamAfterLeave.Members))
	}
	if teamAfterLeave.Leader == nil || teamAfterLeave.Leader.ID != p1ID {
		t.Fatalf("expected leadership to automatically succeed to Alice (p1), got %v", teamAfterLeave.Leader)
	}

	// Step K: Sole member leaves -> Team is dissolved/deleted
	_, err = teamsRepo.LeaveTeam(ctx, compID, p1ID)
	if err != nil {
		t.Fatalf("failed for sole member Alice to leave: %v", err)
	}
	_, err = teamsRepo.GetTeamByID(ctx, teamID)
	if err != ErrTeamNotFound {
		t.Fatalf("expected ErrTeamNotFound after sole member leaves, got %v", err)
	}

	// Step L: Event Banning Eviction & Invariant across competitions in the event
	team2ID, err := teamsRepo.CreateTeam(ctx, compID, p2ID, fmt.Sprintf("Bobs Team %d", ts))
	if err != nil {
		t.Fatalf("failed to create team 2: %v", err)
	}
	team2Obj, err := teamsRepo.GetTeamByID(ctx, team2ID)
	if err != nil {
		t.Fatalf("failed to get team 2: %v", err)
	}
	_, err = teamsRepo.JoinByCode(ctx, compID, p3ID, team2Obj.JoinCode)
	if err != nil {
		t.Fatalf("failed for Charlie to join team 2: %v", err)
	}

	// Ban Bob from the Event (the leader of team 2)
	err = eventsRepo.BanMember(ctx, eventID, p2ID)
	if err != nil {
		t.Fatalf("failed to ban Bob from event: %v", err)
	}

	// Verify Bob was evicted from team 2 and Charlie became the new leader
	team2AfterBan, err := teamsRepo.GetTeamByID(ctx, team2ID)
	if err != nil {
		t.Fatalf("failed to get team 2: %v", err)
	}
	if len(team2AfterBan.Members) != 1 {
		t.Fatalf("expected 1 member after ban eviction, got %d", len(team2AfterBan.Members))
	}
	if team2AfterBan.Leader == nil || team2AfterBan.Leader.ID != p3ID {
		t.Fatalf("expected Charlie to become leader of team 2 after Bob was banned, got %v", team2AfterBan.Leader)
	}

	// Verify Bob cannot create or join teams now (banned from event)
	_, err = teamsRepo.CreateTeam(ctx, compID, p2ID, fmt.Sprintf("Banned Attempt %d", ts))
	if err != ErrUserIsBanned {
		t.Fatalf("expected ErrUserIsBanned on create team, got %v", err)
	}
	_, err = teamsRepo.JoinByCode(ctx, compID, p2ID, team2AfterBan.JoinCode)
	if err != ErrUserIsBanned {
		t.Fatalf("expected ErrUserIsBanned on join team, got %v", err)
	}

	// Ban Charlie (the sole remaining member) -> team 2 dissolved
	err = eventsRepo.BanMember(ctx, eventID, p3ID)
	if err != nil {
		t.Fatalf("failed to ban Charlie: %v", err)
	}
	_, err = teamsRepo.GetTeamByID(ctx, team2ID)
	if err != ErrTeamNotFound {
		t.Fatalf("expected team 2 to be deleted when sole member is banned, got %v", err)
	}

	// Step M: Global Blacklist eviction
	p5ID := createTestUser(t, pool, fmt.Sprintf("p5_%d@example.com", ts), "Eve", "user")
	linkEventMember(t, pool, eventID, p5ID, "participant")
	team3ID, err := teamsRepo.CreateTeam(ctx, compID, p5ID, fmt.Sprintf("Eves Team %d", ts))
	if err != nil {
		t.Fatalf("failed to create team 3: %v", err)
	}
	// Blacklist Eve
	err = usersRepo.EvictUserFromAnyTeam(ctx, p5ID)
	if err != nil {
		t.Fatalf("failed to evict user on blacklist: %v", err)
	}
	_, err = teamsRepo.GetTeamByID(ctx, team3ID)
	if err != ErrTeamNotFound {
		t.Fatalf("expected team 3 to be deleted when Eve was blacklisted, got %v", err)
	}
}

func TestTeamsHTTPRoutes(t *testing.T) {
	pool := newTestDBPool(t)

	ts := time.Now().UnixNano()

	ownerID := createTestUser(t, pool, fmt.Sprintf("http_owner_%d@example.com", ts), "Event Owner", "user")
	eventSlug := fmt.Sprintf("http-event-%d", ts)
	compSlug := fmt.Sprintf("http-comp-%d", ts)
	eventID, compID := createTestEventAndCompetition(t, pool, eventSlug, compSlug, "HTTP Test Comp", ownerID)

	p1ID := createTestUser(t, pool, fmt.Sprintf("http_p1_%d@example.com", ts), "User 1", "user")
	p2ID := createTestUser(t, pool, fmt.Sprintf("http_p2_%d@example.com", ts), "User 2", "user")
	linkEventMember(t, pool, eventID, p1ID, "participant")
	linkEventMember(t, pool, eventID, p2ID, "participant")

	teamsRepo := NewRepository(pool, 4)
	teamsService := NewService(teamsRepo, nil)
	teamsHandler := NewHandler(teamsService, nil)

	// Router setup
	r := chi.NewRouter()
	r.Route("/e/{eventSlug}/c/{competitionSlug}/teams", func(tr chi.Router) {
		RegisterScopedRoutes(tr, teamsHandler)
	})

	withContext := func(userID, role string) context.Context {
		c := context.Background()
		c = auth.WithUserClaims(c, &utils.AccessTokenClaims{
			UserID: userID,
			Role:   role,
		})
		c = events.WithEventContext(c, &events.Context{
			EventID: eventID,
			Slug:    eventSlug,
			UserID:  userID,
			Role:    role,
		})
		c = competitions.WithCompetitionContext(c, &competitions.Context{
			CompetitionID: compID,
			EventID:       eventID,
			Slug:          compSlug,
			UserID:        userID,
			Role:          role,
		})
		return c
	}

	baseURL := "/e/" + eventSlug + "/c/" + compSlug + "/teams"

	// 1. Participant 1 creates a team
	createBody := bytes.NewBufferString(`{"name":"HTTP Rockets"}`)
	req := httptest.NewRequest(http.MethodPost, baseURL, createBody).WithContext(withContext(p1ID, "participant"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on team creation, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var createResp TeamDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if createResp.JoinCode == "" {
		t.Fatalf("expected non-empty join code")
	}

	// 2. Participant 2 joins by code
	joinBody := bytes.NewBufferString(fmt.Sprintf(`{"code":"%s"}`, createResp.JoinCode))
	req = httptest.NewRequest(http.MethodPost, baseURL+"/join", joinBody).WithContext(withContext(p2ID, "participant"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on join, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 3. Participant 1 transfers leadership to Participant 2
	transferBody := bytes.NewBufferString(fmt.Sprintf(`{"new_leader_id":"%s"}`, p2ID))
	req = httptest.NewRequest(http.MethodPost, baseURL+"/my-team/transfer-leadership", transferBody).WithContext(withContext(p1ID, "participant"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on transfer leadership, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 4. Verify my-team for Participant 2 shows them as leader
	req = httptest.NewRequest(http.MethodGet, baseURL+"/my-team", nil).WithContext(withContext(p2ID, "participant"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on my-team, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var myTeamResp TeamDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &myTeamResp); err != nil {
		t.Fatalf("failed to decode my-team: %v", err)
	}
	if myTeamResp.Leader == nil || myTeamResp.Leader.ID != p2ID {
		t.Fatalf("expected p2 to be leader in my-team, got %v", myTeamResp.Leader)
	}
}
