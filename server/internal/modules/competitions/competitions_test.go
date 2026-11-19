package competitions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"devclubxnst.com/cadence/pkg/utils"
)

func TestRequireCompetitionRoles(t *testing.T) {
	adminOrOwnerOnly := RequireCompetitionRoles("owner", "admin")

	testHandler := adminOrOwnerOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compCtx, ok := FromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(compCtx.Role + ":" + compCtx.CompetitionID))
	}))

	t.Run("missing competition context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("owner role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-owner",
			Role:          "owner",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "owner:comp-1" {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("admin role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-admin",
			Role:          "admin",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("judge role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-judge",
			Role:          "judge",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("participant role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-part",
			Role:          "participant",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})
}

func TestSlugRegexValidation(t *testing.T) {
	validSlugs := []string{
		"hack-2026",
		"devclub-hackathon",
		"ai-challenge-1",
		"delhi-hacks",
		"h1",
	}

	for _, s := range validSlugs {
		if !slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be valid", s)
		}
	}

	invalidSlugs := []string{
		"Hack-2026",       // uppercase
		"hack_2026",       // underscore
		"-hack",           // leading hyphen
		"hack-",           // trailing hyphen
		"hack--2026",      // consecutive hyphens
		"hack 2026",       // space
		"hack.2026",       // period
	}

	for _, s := range invalidSlugs {
		if slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be invalid", s)
		}
	}
}

func TestRoleHierarchyRules(t *testing.T) {
	svc := &Service{}

	t.Run("admin cannot assign admin role", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-admin", "admin", "target-user", "admin")
		if err != ErrPermissionDenied {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	})

	t.Run("owner cannot assign owner role via update endpoint", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-owner", "owner", "target-user", "owner")
		if err == nil || err.Error() != "cannot assign owner role; use transfer ownership endpoint" {
			t.Fatalf("expected cannot assign owner role error, got %v", err)
		}
	})

	t.Run("owner cannot self-demote via role update", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-owner", "owner", "actor-owner", "participant")
		if err != ErrCannotSelfDemote {
			t.Fatalf("expected ErrCannotSelfDemote, got %v", err)
		}
	})
}

func TestValidationDTOs(t *testing.T) {
	t.Run("valid CreateCompetitionRequest passes", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug: "hack-2026",
			Name: "Hack 2026",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation errors: %v", errs)
		}
	})

	t.Run("invalid slug too short fails", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug: "a",
			Name: "Hack 2026",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for short slug")
		}
	})

	t.Run("invalid TransferOwnershipRequest missing UUID fails", func(t *testing.T) {
		req := TransferOwnershipRequest{
			NewOwnerUserID: "not-a-uuid",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for invalid UUID")
		}
	})

	t.Run("valid TransferOwnershipRequest with UUID passes", func(t *testing.T) {
		req := TransferOwnershipRequest{
			NewOwnerUserID: "00000000-0000-0000-0000-000000000001",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation error: %v", errs)
		}
	})
}
