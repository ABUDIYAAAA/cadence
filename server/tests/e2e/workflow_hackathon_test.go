package e2e_test

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/judging"
	"devclubxnst.com/cadence/internal/modules/notifications"
	"devclubxnst.com/cadence/internal/modules/teams"
	"devclubxnst.com/cadence/internal/modules/tickets"
	"devclubxnst.com/cadence/internal/modules/users"
	"devclubxnst.com/cadence/pkg/audit"
)

func TestStage3_FullHackathonLifecycleWorkflow(t *testing.T) {
	ResetState(t)

	admin := LoginAsAdmin(t)

	// =========================================================================
	// PHASE 1: Bulk & Single User Creation +Every Role Onboarding via Mailpit
	// =========================================================================
	var bulkResp users.BulkCreateResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{
				"email":       "judge1@cadence.local",
				"name":        "Dr. Ada Lovelace",
				"role":        "judge",
				"send_invite": true,
			},
			{
				"email":       "judge2@cadence.local",
				"name":        "Dr. Alan Turing",
				"role":        "judge",
				"send_invite": true,
			},
			{
				"email":       "volunteer1@cadence.local",
				"name":        "Grace Hopper",
				"role":        "volunteer",
				"send_invite": true,
			},
			{
				"email":       "alpha.leader@cadence.local",
				"name":        "Alpha Leader",
				"role":        "participant",
				"team_name":   "Team Alpha",
				"is_leader":   true,
				"send_invite": true,
			},
			{
				"email":       "alpha.member@cadence.local",
				"name":        "Alpha Member",
				"role":        "participant",
				"team_name":   "Team Alpha",
				"is_leader":   false,
				"send_invite": true,
			},
			{
				"email":       "beta.leader@cadence.local",
				"name":        "Beta Leader",
				"role":        "participant",
				"team_name":   "Team Beta",
				"is_leader":   true,
				"send_invite": true,
			},
		},
	}, http.StatusCreated, &bulkResp)

	if bulkResp.TotalCreated != 6 || bulkResp.InvitesDispatched != 6 {
		t.Fatalf("expected 6 users created and 6 invites dispatched, got created=%d invites=%d (errors: %v)",
			bulkResp.TotalCreated, bulkResp.InvitesDispatched, bulkResp.SkippedErrors)
	}

	// Onboard all 6 users by extracting their real setup tokens from Mailpit HTML emails
	judge1 := OnboardUserViaMailpit(t, "Judge1", "judge1@cadence.local", "Judge1Pass123!")
	judge2 := OnboardUserViaMailpit(t, "Judge2", "judge2@cadence.local", "Judge2Pass123!")
	volunteer := OnboardUserViaMailpit(t, "Volunteer1", "volunteer1@cadence.local", "VolunteerPass123!")
	alphaLeader := OnboardUserViaMailpit(t, "AlphaLeader", "alpha.leader@cadence.local", "AlphaLead123!")
	alphaMember := OnboardUserViaMailpit(t, "AlphaMember", "alpha.member@cadence.local", "AlphaMem123!")
	betaLeader := OnboardUserViaMailpit(t, "BetaLeader", "beta.leader@cadence.local", "BetaLead123!")

	// =========================================================================
	// PHASE 2: Team Discovery, Roster Management & Self-Lookup
	// =========================================================================
	var alphaTeam teams.TeamDetailResponse
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/teams/my-team", nil, http.StatusOK, &alphaTeam)
	if alphaTeam.Name != "Team Alpha" || len(alphaTeam.Members) != 2 {
		t.Fatalf("expected Team Alpha with 2 members, got %+v", alphaTeam)
	}
	if alphaTeam.Leader == nil || alphaTeam.Leader.ID != alphaLeader.UserID {
		t.Fatalf("expected AlphaLeader (%s) to be leader of Team Alpha, got %+v", alphaLeader.UserID, alphaTeam.Leader)
	}

	var betaTeam teams.TeamDetailResponse
	betaLeader.DoJSON(t, http.MethodGet, "/api/v1/teams/my-team", nil, http.StatusOK, &betaTeam)
	if betaTeam.Name != "Team Beta" || len(betaTeam.Members) != 1 {
		t.Fatalf("expected Team Beta with 1 member, got %+v", betaTeam)
	}

	// Admin creates a temporary user, adds them to Team Beta, verifies, then removes them
	var tempUser users.UserDetailResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "temp.beta@cadence.local",
		"name":        "Temp Beta Member",
		"role":        "participant",
		"send_invite": false,
	}, http.StatusCreated, &tempUser)

	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/teams/%s/members", betaTeam.ID), map[string]string{
		"user_id": tempUser.ID,
	}, http.StatusOK, nil)

	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/teams/%s", betaTeam.ID), nil, http.StatusOK, &betaTeam)
	if len(betaTeam.Members) != 2 {
		t.Fatalf("expected 2 members on Team Beta after adding member, got %d", len(betaTeam.Members))
	}

	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/teams/%s/members/%s", betaTeam.ID, tempUser.ID), nil, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/users/%s", tempUser.ID), nil, http.StatusOK, nil)

	// =========================================================================
	// PHASE 3: Ticket Issuance, Mailpit PNG Attachment Verification & Gate Check-In
	// =========================================================================
	// 3a. Admin issues single ticket to AlphaLeader
	var singleTktResp tickets.SingleSendTicketResponse
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tickets/users/%s/send", alphaLeader.UserID), nil, http.StatusOK, &singleTktResp)
	if !strings.HasPrefix(singleTktResp.TicketCode, "cadence:tkt:") {
		t.Fatalf("expected HMAC signed ticket code starting with cadence:tkt:, got %q", singleTktResp.TicketCode)
	}

	// Verify AlphaLeader's ticket email & 300x300 QR PNG attachment in Mailpit
	emailedCodeAlpha, qrBytesAlpha := VerifyAndDownloadTicketEmail(t, alphaLeader.Email)
	if emailedCodeAlpha != singleTktResp.TicketCode {
		t.Fatalf("expected emailed ticket code %q to match API response %q", emailedCodeAlpha, singleTktResp.TicketCode)
	}
	if len(qrBytesAlpha) == 0 {
		t.Fatal("expected non-empty QR PNG bytes from Mailpit attachment")
	}

	// 3b. AlphaLeader checks GET /api/v1/tickets/me
	var myTicket tickets.MyTicketResponse
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/tickets/me", nil, http.StatusOK, &myTicket)
	if myTicket.TicketCode != singleTktResp.TicketCode || myTicket.IsCheckedIn {
		t.Fatalf("unexpected /tickets/me state before check-in: %+v", myTicket)
	}
	if !strings.HasPrefix(myTicket.QRCodeBase64, "data:image/png;base64,") {
		t.Fatalf("expected base64 PNG data URI in /tickets/me, got prefix %q", myTicket.QRCodeBase64[:30])
	}

	// 3c. Admin bulk-dispatches tickets to all participants
	var bulkTktResp tickets.BulkSendTicketsResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/bulk-send", map[string]any{
		"role":            "participant",
		"skip_checked_in": true,
	}, http.StatusOK, &bulkTktResp)
	if bulkTktResp.Enqueued < 2 {
		t.Fatalf("expected at least 2 participant tickets enqueued in bulk-send, got %+v", bulkTktResp)
	}

	emailedCodeBeta, _ := VerifyAndDownloadTicketEmail(t, betaLeader.Email)
	_ = VerifyAndDownloadTicketEmail

	// 3d. Volunteer scans AlphaLeader and BetaLeader tickets at registration gate
	var checkInAlpha tickets.CheckInResponse
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{
		"ticket_code": emailedCodeAlpha,
	}, http.StatusOK, &checkInAlpha)
	if !checkInAlpha.Success || checkInAlpha.Attendee.ID != alphaLeader.UserID {
		t.Fatalf("unexpected check-in response for AlphaLeader: %+v", checkInAlpha)
	}

	var checkInBeta tickets.CheckInResponse
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{
		"ticket_code": emailedCodeBeta,
	}, http.StatusOK, &checkInBeta)

	// 3e. Duplicate scan on AlphaLeader's ticket must return 409 Conflict with prior check-in details
	var dupResp tickets.DuplicateCheckInResponse
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{
		"ticket_code": emailedCodeAlpha,
	}, http.StatusConflict, &dupResp)
	if dupResp.Attendee.Email != alphaLeader.Email || dupResp.CheckedInAt == "" {
		t.Fatalf("expected duplicate check-in details for AlphaLeader, got %+v", dupResp)
	}

	// 3f. Verify Attendance Analytics (Alpha = partial [1/2], Beta = complete [1/1])
	var analyticsResp tickets.AnalyticsResponse
	volunteer.DoJSON(t, http.MethodGet, "/api/v1/tickets/analytics", nil, http.StatusOK, &analyticsResp)
	if analyticsResp.CheckedInCount != 2 {
		t.Fatalf("expected CheckedInCount=2, got %d", analyticsResp.CheckedInCount)
	}
	teamStatusMap := make(map[string]string)
	for _, tc := range analyticsResp.TeamCompleteness {
		teamStatusMap[tc.TeamName] = tc.Status
	}
	if teamStatusMap["Team Alpha"] != "partial" || teamStatusMap["Team Beta"] != "complete" {
		t.Fatalf("expected Team Alpha=partial and Team Beta=complete in analytics, got %+v", teamStatusMap)
	}

	// =========================================================================
	// PHASE 4: Full Multi-Role Judging Lifecycle & Leaderboard Math
	// =========================================================================
	// 4a. Admin creates judging round with weighted criteria (Innovation weight 2.0, Execution weight 1.0)
	var createRoundResp struct {
		RoundID string `json:"round_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/rounds", map[string]any{
		"name":               "Grand Finale Round",
		"buffer_size":        1,
		"slot_duration_mins": 15,
		"criteria": []map[string]any{
			{
				"name":        "Innovation",
				"description": "Originality and technical novelty",
				"max_score":   10,
				"weight":      2.0,
				"order_index": 1,
			},
			{
				"name":        "Execution",
				"description": "Completeness and polish",
				"max_score":   10,
				"weight":      1.0,
				"order_index": 2,
			},
		},
	}, http.StatusCreated, &createRoundResp)
	roundID := createRoundResp.RoundID

	// Activate round
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/activate", roundID), nil, http.StatusOK, nil)

	// 4b. Admin creates Panel in Room 101 with Judge 1 and Judge 2
	var createPanelResp struct {
		PanelID string `json:"panel_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/panels", map[string]any{
		"round_id":  roundID,
		"name":      "AI & Systems Panel",
		"room":      "Room 101",
		"judge_ids": []string{judge1.UserID, judge2.UserID},
	}, http.StatusCreated, &createPanelResp)
	panelID := createPanelResp.PanelID

	// 4c. Admin configures sequential slots: Slot 1 = Team Alpha, Slot 2 = Team Beta
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/panels/%s/slots", panelID), map[string]any{
		"slots": []map[string]any{
			{
				"team_id":        alphaTeam.ID,
				"sequence_order": 1,
				"submission_links": map[string]string{
					"github": "https://github.com/cadence/alpha-initial",
				},
			},
			{
				"team_id":        betaTeam.ID,
				"sequence_order": 2,
				"submission_links": map[string]string{
					"github": "https://github.com/cadence/beta-repo",
				},
			},
		},
	}, http.StatusOK, nil)

	// 4d. Judge 1 fetches panels to get slot IDs and criterion IDs
	var judgePanels []judging.JudgePanelSummary
	judge1.DoJSON(t, http.MethodGet, "/api/v1/judging/judge/panels", nil, http.StatusOK, &judgePanels)
	if len(judgePanels) != 1 || len(judgePanels[0].Slots) != 2 || len(judgePanels[0].Criteria) != 2 {
		t.Fatalf("expected 1 panel with 2 slots and 2 criteria, got %+v", judgePanels)
	}
	slotAlphaID := judgePanels[0].Slots[0].SlotID
	slotBetaID := judgePanels[0].Slots[1].SlotID
	critInnovationID := judgePanels[0].Criteria[0].ID
	critExecutionID := judgePanels[0].Criteria[1].ID

	// 4e. AlphaLeader updates their team's submission links, and is forbidden from updating Team Beta's slot
	alphaLeader.DoJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/judging/slots/%s/links", slotAlphaID), map[string]any{
		"submission_links": map[string]string{
			"github": "https://github.com/cadence/alpha-final",
			"demo":   "https://alpha.cadence.local",
		},
	}, http.StatusOK, nil)

	alphaLeader.DoJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/judging/slots/%s/links", slotBetaID), map[string]any{
		"submission_links": map[string]string{
			"github": "https://github.com/cadence/tampered",
		},
	}, http.StatusForbidden, nil)

	// 4f. Judge 1 starts Slot 1 (Team Alpha) -> Team Alpha becomes 'in_progress', Team Beta becomes 'queued' (buffer_size=1)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/start", slotAlphaID), nil, http.StatusOK, nil)

	// Verify Participant live status for Team Alpha (in_progress) and Team Beta (queued)
	var alphaStatus judging.TeamStatusResponse
	alphaMember.DoJSON(t, http.MethodGet, "/api/v1/judging/teams/my-status", nil, http.StatusOK, &alphaStatus)
	if !alphaStatus.HasActiveRound || alphaStatus.Status != "in_progress" || alphaStatus.Room != "Room 101" {
		t.Fatalf("expected Team Alpha status in_progress in Room 101, got %+v", alphaStatus)
	}
	if alphaStatus.SubmissionLinks["github"] != "https://github.com/cadence/alpha-final" {
		t.Fatalf("expected updated github link on Team Alpha status, got %+v", alphaStatus.SubmissionLinks)
	}

	var betaStatus judging.TeamStatusResponse
	betaLeader.DoJSON(t, http.MethodGet, "/api/v1/judging/teams/my-status", nil, http.StatusOK, &betaStatus)
	if !betaStatus.HasActiveRound || betaStatus.Status != "queued" {
		t.Fatalf("expected Team Beta status queued (on-deck), got %+v", betaStatus)
	}

	// Verify Volunteer room schedule view reflects ActiveTeam=Team Alpha and QueuedTeams=[Team Beta]
	var volSchedule []judging.VolunteerPanelSchedule
	volunteer.DoJSON(t, http.MethodGet, "/api/v1/judging/volunteer/schedule", nil, http.StatusOK, &volSchedule)
	if len(volSchedule) != 1 || volSchedule[0].ActiveTeam == nil || volSchedule[0].ActiveTeam.TeamID != alphaTeam.ID {
		t.Fatalf("expected Volunteer schedule to show Team Alpha active, got %+v", volSchedule)
	}
	if len(volSchedule[0].QueuedTeams) != 1 || volSchedule[0].QueuedTeams[0].TeamID != betaTeam.ID {
		t.Fatalf("expected Volunteer schedule to show Team Beta queued, got %+v", volSchedule[0].QueuedTeams)
	}

	// 4g. Judge 1 submits & locks scores for Team Alpha (Innovation=8, Execution=9)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotAlphaID), map[string]any{
		"scores": []map[string]any{
			{"criterion_id": critInnovationID, "score": 8.0},
			{"criterion_id": critExecutionID, "score": 9.0},
		},
	}, http.StatusOK, nil)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/lock", slotAlphaID), nil, http.StatusOK, nil)

	// Attempting to edit scores after locking must fail with 400 Bad Request
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotAlphaID), map[string]any{
		"scores": []map[string]any{
			{"criterion_id": critInnovationID, "score": 10.0},
		},
	}, http.StatusBadRequest, nil)

	// Completing slot before Judge 2 has locked must return 409 Conflict (ErrNotAllJudgesLocked)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/complete", slotAlphaID), nil, http.StatusConflict, nil)

	// 4h. Judge 2 submits & locks scores for Team Alpha (Innovation=6, Execution=7) -> Complete slot succeeds
	judge2.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotAlphaID), map[string]any{
		"scores": []map[string]any{
			{"criterion_id": critInnovationID, "score": 6.0},
			{"criterion_id": critExecutionID, "score": 7.0},
		},
	}, http.StatusOK, nil)
	judge2.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/lock", slotAlphaID), nil, http.StatusOK, nil)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/complete", slotAlphaID), nil, http.StatusOK, nil)

	// 4i. Evaluate & complete Slot 2 (Team Beta):
	// Judge 1: Innovation=9, Execution=10 | Judge 2: Innovation=9, Execution=8
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/start", slotBetaID), nil, http.StatusOK, nil)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotBetaID), map[string]any{
		"scores": []map[string]any{
			{"criterion_id": critInnovationID, "score": 9.0},
			{"criterion_id": critExecutionID, "score": 10.0},
		},
	}, http.StatusOK, nil)
	judge1.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/lock", slotBetaID), nil, http.StatusOK, nil)

	judge2.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotBetaID), map[string]any{
		"scores": []map[string]any{
			{"criterion_id": critInnovationID, "score": 9.0},
			{"criterion_id": critExecutionID, "score": 8.0},
		},
	}, http.StatusOK, nil)
	judge2.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/lock", slotBetaID), nil, http.StatusOK, nil)
	judge2.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/complete", slotBetaID), nil, http.StatusOK, nil)

	// 4j. Admin verifies weighted Leaderboard rankings:
	// Team Beta: AVG(Innovation)=9.0 * 2.0 + AVG(Execution)=9.0 * 1.0 = 27.0 (Rank 1)
	// Team Alpha: AVG(Innovation)=7.0 * 2.0 + AVG(Execution)=8.0 * 1.0 = 22.0 (Rank 2)
	var leaderboard judging.AdminLeaderboardResponse
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s/leaderboard", roundID), nil, http.StatusOK, &leaderboard)
	if leaderboard.TotalTeams != 2 || leaderboard.GradedTeams != 2 || len(leaderboard.Leaderboard) != 2 {
		t.Fatalf("expected 2 graded teams on leaderboard, got %+v", leaderboard)
	}
	first := leaderboard.Leaderboard[0]
	second := leaderboard.Leaderboard[1]
	if first.TeamID != betaTeam.ID || first.Rank != 1 || math.Abs(first.TotalWeightedScore-27.0) > 0.01 {
		t.Fatalf("expected Rank 1 Team Beta with weighted score 27.0, got %+v", first)
	}
	if second.TeamID != alphaTeam.ID || second.Rank != 2 || math.Abs(second.TotalWeightedScore-22.0) > 0.01 {
		t.Fatalf("expected Rank 2 Team Alpha with weighted score 22.0, got %+v", second)
	}

	// =========================================================================
	// PHASE 5: Notifications Broadcast (Global, Role, Team, User) & Inbox Lifecycle
	// =========================================================================
	// 5a. Public VAPID key check
	var vapidResp map[string]string
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/notifications/vapid-public-key", nil, http.StatusOK, &vapidResp)
	if vapidResp["public_key"] != TestVAPIDPublicKey {
		t.Fatalf("expected VAPID public key %q, got %q", TestVAPIDPublicKey, vapidResp["public_key"])
	}

	// 5b. Admin broadcasts to all users ('broadcast')
	var bcastAll notifications.BroadcastNotificationResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{
		"title":       "Hackathon Kickoff",
		"body":        "Welcome to Cadence Hackathon!",
		"target_type": "broadcast",
	}, http.StatusCreated, &bcastAll)
	if bcastAll.RecipientsCount != 7 { // 1 admin + 2 judges + 1 volunteer + 3 participants
		t.Fatalf("expected 7 broadcast recipients, got %d", bcastAll.RecipientsCount)
	}

	// 5c. Admin broadcasts to role='judge'
	var bcastJudges notifications.BroadcastNotificationResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{
		"title":       "Judges Briefing",
		"body":        "Please report to Room 101",
		"target_type": "role",
		"target_id":   "judge",
	}, http.StatusCreated, &bcastJudges)
	if bcastJudges.RecipientsCount != 2 {
		t.Fatalf("expected 2 judge recipients, got %d", bcastJudges.RecipientsCount)
	}

	// 5d. Admin broadcasts to team=Team Alpha
	var bcastTeam notifications.BroadcastNotificationResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{
		"title":       "Team Alpha On Deck",
		"body":        "Head to Room 101",
		"target_type": "team",
		"target_id":   alphaTeam.ID,
	}, http.StatusCreated, &bcastTeam)
	if bcastTeam.RecipientsCount != 2 {
		t.Fatalf("expected 2 Team Alpha recipients, got %d", bcastTeam.RecipientsCount)
	}

	// 5e. AlphaLeader checks inbox: should have 2 notifications (Global + Team Alpha), NOT Judges Briefing
	var alphaInbox notifications.InboxResponse
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/notifications/inbox", nil, http.StatusOK, &alphaInbox)
	if alphaInbox.UnreadCount != 2 || len(alphaInbox.Notifications) != 2 {
		t.Fatalf("expected 2 unread notifications for AlphaLeader, got %+v", alphaInbox)
	}

	// Mark 1 notification as read, then mark all as read
	alphaLeader.DoJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%s/read", alphaInbox.Notifications[0].ID), nil, http.StatusOK, nil)
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/notifications/inbox", nil, http.StatusOK, &alphaInbox)
	if alphaInbox.UnreadCount != 1 {
		t.Fatalf("expected 1 unread notification after marking first read, got %d", alphaInbox.UnreadCount)
	}

	alphaLeader.DoJSON(t, http.MethodPost, "/api/v1/notifications/read-all", nil, http.StatusOK, nil)
	alphaLeader.DoJSON(t, http.MethodGet, "/api/v1/notifications/inbox", nil, http.StatusOK, &alphaInbox)
	if alphaInbox.UnreadCount != 0 {
		t.Fatalf("expected 0 unread notifications after read-all, got %d", alphaInbox.UnreadCount)
	}

	// =========================================================================
	// PHASE 6: Asynchronous Audit Log Verification & CSV Export
	// =========================================================================
	requiredActions := []string{
		audit.ActionUserBulkImport,
		audit.ActionTicketIssued,
		audit.ActionTicketCheckIn,
		audit.ActionTicketDuplicateScan,
		audit.ActionJudgingRoundCreate,
		audit.ActionJudgingRoundActivate,
		audit.ActionJudgingSlotStart,
		audit.ActionJudgingScoreLock,
		audit.ActionJudgingSlotComplete,
		audit.ActionNotificationBroadcast,
	}

	var seenActions map[string]bool
	WaitForCondition(
		t,
		10*time.Second,
		150*time.Millisecond,
		"all hackathon workflow audit actions persisted in PostgreSQL",
		func() (bool, error) {
			var auditList struct {
				Data []struct {
					Action string `json:"action"`
				} `json:"data"`
			}
			admin.DoJSON(t, http.MethodGet, "/api/v1/audit-logs?limit=200", nil, http.StatusOK, &auditList)
			seenActions = make(map[string]bool)
			for _, item := range auditList.Data {
				seenActions[item.Action] = true
			}
			for _, reqAct := range requiredActions {
				if !seenActions[reqAct] {
					return false, nil
				}
			}
			return true, nil
		},
		func() string {
			var keys []string
			for k := range seenActions {
				keys = append(keys, k)
			}
			return fmt.Sprintf("Seen actions: %v, Required: %v", keys, requiredActions)
		},
	)

	// Verify CSV Export endpoint
	csvResp, csvBytes := admin.DoRaw(t, http.MethodGet, "/api/v1/audit-logs/export", nil, nil)
	if csvResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/audit-logs/export, got %d", csvResp.StatusCode)
	}
	if !strings.HasPrefix(csvResp.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("expected text/csv content-type on export, got %q", csvResp.Header.Get("Content-Type"))
	}
	records, err := csv.NewReader(strings.NewReader(string(csvBytes))).ReadAll()
	if err != nil {
		t.Fatalf("failed to parse exported audit CSV: %v", err)
	}
	if len(records) < 15 {
		t.Fatalf("expected at least 15 rows in exported audit CSV (header + events), got %d", len(records))
	}
	if records[0][0] != "ID" || records[0][2] != "Action" {
		t.Fatalf("unexpected CSV header row: %v", records[0])
	}
}
