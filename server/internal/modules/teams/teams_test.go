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
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type captureAuditRecorder struct {
	events []audit.Event
}

func (c *captureAuditRecorder) Record(_ context.Context, ev audit.Event) {
	c.events = append(c.events, ev)
}

func (c *captureAuditRecorder) hasActionForTarget(action, targetID string) bool {
	for _, ev := range c.events {
		if ev.Action == action && ev.TargetID != nil && *ev.TargetID == targetID {
			return true
		}
	}
	return false
}

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

	teamsRepo := NewRepository(pool)
	eventsRepo := events.NewRepository(pool).WithTeamEvictor(teamsRepo)
	usersRepo := users.NewRepository(pool).WithTeamEvictor(teamsRepo)

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

	teamsRepo := NewRepository(pool)
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

	// 5. Create Competition 2 in the SAME Event and verify per-competition settings isolation & cross-competition team ID 404
	comp2Slug := fmt.Sprintf("http-comp2-%d", ts)
	var comp2ID string
	if err := pool.QueryRow(context.Background(), `INSERT INTO competitions (event_id, slug, name) VALUES ($1, $2, 'HTTP Comp 2') RETURNING id`, eventID, comp2Slug).Scan(&comp2ID); err != nil {
		t.Fatalf("create comp 2: %v", err)
	}
	withComp2Context := func(userID, role string) context.Context {
		c := context.Background()
		c = auth.WithUserClaims(c, &utils.AccessTokenClaims{UserID: userID, Role: "user"})
		c = events.WithEventContext(c, &events.Context{EventID: eventID, Slug: eventSlug, UserID: userID, Role: role})
		c = competitions.WithCompetitionContext(c, &competitions.Context{CompetitionID: comp2ID, EventID: eventID, Slug: comp2Slug, UserID: userID, Role: role})
		return c
	}
	comp2URL := "/e/" + eventSlug + "/c/" + comp2Slug + "/teams"

	// Accessing Competition 1's team ID under Competition 2's URL must return 404 Not Found
	req = httptest.NewRequest(http.MethodGet, comp2URL+"/"+createResp.ID, nil).WithContext(withComp2Context(ownerID, "owner"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when fetching Comp 1 team under Comp 2 URL, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, comp2URL+"/"+createResp.ID+"/freeze", nil).WithContext(withComp2Context(ownerID, "owner"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found when freezing Comp 1 team under Comp 2 URL, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Freezing all teams and changing max_team_size in Comp 1 must NOT affect Comp 2
	req = httptest.NewRequest(http.MethodPost, baseURL+"/freeze-all", nil).WithContext(withContext(ownerID, "owner"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on freeze-all in Comp 1, got %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPatch, baseURL+"/settings", bytes.NewBufferString(`{"max_team_size":2}`)).WithContext(withContext(ownerID, "owner"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on settings update in Comp 1, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, comp2URL+"/settings", nil).WithContext(withComp2Context(ownerID, "owner"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get settings in Comp 2, got %d", rec.Code)
	}
	var comp2Settings SettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &comp2Settings); err != nil {
		t.Fatalf("decode comp2 settings: %v", err)
	}
	if comp2Settings.FrozenAll {
		t.Fatalf("expected Comp 2 FrozenAll=false when Comp 1 is frozen, got true")
	}
	if comp2Settings.MaxTeamSize != 4 {
		t.Fatalf("expected Comp 2 MaxTeamSize=4 (default), got %d", comp2Settings.MaxTeamSize)
	}
}

func TestTeamOwnershipEvictionAuditAndCompetitionScopedLookup(t *testing.T) {
	pool := newTestDBPool(t)
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// 1. Verify redundant users.team_id column is dropped from the schema
	var colCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'team_id'
	`).Scan(&colCount); err != nil {
		t.Fatalf("check users.team_id column: %v", err)
	}
	if colCount != 0 {
		t.Fatalf("expected users.team_id column to be dropped (0 rows in information_schema.columns), got %d", colCount)
	}

	// 2. Setup event with TWO competitions
	ownerID := createTestUser(t, pool, fmt.Sprintf("own_evict_%d@example.com", ts), "Owner", "user")
	eventSlug := fmt.Sprintf("ev-evict-%d", ts)
	comp1Slug := fmt.Sprintf("comp1-evict-%d", ts)
	comp2Slug := fmt.Sprintf("comp2-evict-%d", ts)
	eventID, comp1ID := createTestEventAndCompetition(t, pool, eventSlug, comp1Slug, "Comp 1", ownerID)

	var comp2ID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO competitions (event_id, slug, name)
		VALUES ($1, $2, 'Comp 2')
		RETURNING id
	`, eventID, comp2Slug).Scan(&comp2ID); err != nil {
		t.Fatalf("create comp 2: %v", err)
	}

	u1ID := createTestUser(t, pool, fmt.Sprintf("u1_evict_%d@example.com", ts), "Leader One", "user")
	u2ID := createTestUser(t, pool, fmt.Sprintf("u2_evict_%d@example.com", ts), "Member Two", "user")
	u3ID := createTestUser(t, pool, fmt.Sprintf("u3_evict_%d@example.com", ts), "Solo Three", "user")
	linkEventMember(t, pool, eventID, u1ID, "participant")
	linkEventMember(t, pool, eventID, u2ID, "participant")
	linkEventMember(t, pool, eventID, u3ID, "participant")

	auditor := &captureAuditRecorder{}
	teamsRepo := NewRepository(pool).WithAuditor(auditor)
	eventsRepo := events.NewRepository(pool).WithTeamEvictor(teamsRepo)
	eventsService := events.NewService(eventsRepo, nil, auditor)
	usersRepo := users.NewRepository(pool).WithTeamEvictor(teamsRepo)
	usersService := users.NewService(usersRepo, nil, nil, auditor)

	// u1 creates Team A in Comp 1 (joined by u2) and Team B in Comp 2 (later Timestamp!)
	team1Name := fmt.Sprintf("Alpha Comp1 %d", ts)
	team1ID, err := teamsRepo.CreateTeam(ctx, comp1ID, u1ID, team1Name)
	if err != nil {
		t.Fatalf("create team1 in comp1: %v", err)
	}
	team1Detail, err := teamsRepo.GetTeamByID(ctx, team1ID)
	if err != nil {
		t.Fatalf("get team1: %v", err)
	}
	if _, err := teamsRepo.JoinByCode(ctx, comp1ID, u2ID, team1Detail.JoinCode); err != nil {
		t.Fatalf("u2 join team1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	team2Name := fmt.Sprintf("Beta Comp2 %d", ts)
	team2ID, err := teamsRepo.CreateTeam(ctx, comp2ID, u1ID, team2Name)
	if err != nil {
		t.Fatalf("create team2 in comp2: %v", err)
	}
	_ = team2ID

	// 3. Verify events.ListEventUsers scopes the user's team lookup to filter.CompetitionID
	listComp1, err := eventsService.ListEventUsers(ctx, eventID, events.EventUserFilter{
		CompetitionID: comp1ID,
		Search:        "Leader One",
	})
	if err != nil {
		t.Fatalf("ListEventUsers for comp1: %v", err)
	}
	if len(listComp1.Users) != 1 {
		t.Fatalf("expected 1 user matching search, got %d", len(listComp1.Users))
	}
	if listComp1.Users[0].TeamID == nil || *listComp1.Users[0].TeamID != team1ID {
		t.Fatalf("expected ListEventUsers(CompetitionID=comp1) to return comp1 team %s, got %v (%v)",
			team1ID, listComp1.Users[0].TeamID, listComp1.Users[0].TeamName)
	}

	// 4. Banning u1 via eventsService.BanMember must transfer leadership of Team 1 to u2,
	// dissolve Team 2 (where u1 was sole member), and emit audit.ActionTeamUpdate and audit.ActionTeamDelete!
	if err := eventsService.BanMember(ctx, eventID, ownerID, "owner", u1ID, nil); err != nil {
		t.Fatalf("eventsService.BanMember(u1): %v", err)
	}
	if !auditor.hasActionForTarget(audit.ActionTeamUpdate, team1ID) {
		t.Fatalf("expected BanMember to emit %s audit event for leadership succession on team %s; got events: %+v",
			audit.ActionTeamUpdate, team1ID, auditor.events)
	}
	if !auditor.hasActionForTarget(audit.ActionTeamDelete, team2ID) {
		t.Fatalf("expected BanMember to emit %s audit event for empty-team dissolution on team %s; got events: %+v",
			audit.ActionTeamDelete, team2ID, auditor.events)
	}

	// 5. Blacklisting u3 via usersService.BlacklistUser must dissolve u3's solo team and emit audit.ActionTeamDelete!
	team3ID, err := teamsRepo.CreateTeam(ctx, comp1ID, u3ID, fmt.Sprintf("Gamma Solo %d", ts))
	if err != nil {
		t.Fatalf("create team3: %v", err)
	}
	if err := usersService.BlacklistUser(ctx, u3ID); err != nil {
		t.Fatalf("usersService.BlacklistUser(u3): %v", err)
	}
	if !auditor.hasActionForTarget(audit.ActionTeamDelete, team3ID) {
		t.Fatalf("expected BlacklistUser to emit %s audit event for dissolved team %s; got events: %+v",
			audit.ActionTeamDelete, team3ID, auditor.events)
	}
}

