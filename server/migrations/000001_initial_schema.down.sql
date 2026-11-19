-- ==============================================================================
-- CADENCE V2 CONSOLIDATED CORE SCHEMA ROLLBACK
-- ==============================================================================

DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS notification_recipients CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS push_subscriptions CASCADE;
DROP TABLE IF EXISTS app_settings CASCADE;
DROP TABLE IF EXISTS scan_events CASCADE;
DROP TABLE IF EXISTS scan_sessions CASCADE;
DROP TABLE IF EXISTS tickets CASCADE;
DROP TABLE IF EXISTS judging_judge_metrics CASCADE;
DROP TABLE IF EXISTS judging_score_events CASCADE;
DROP TABLE IF EXISTS judging_round_teams CASCADE;
DROP TABLE IF EXISTS judging_scores CASCADE;
DROP TABLE IF EXISTS judging_slots CASCADE;
DROP TABLE IF EXISTS judging_panel_judges CASCADE;
DROP TABLE IF EXISTS judging_panels CASCADE;
DROP TABLE IF EXISTS judging_criteria CASCADE;
DROP TABLE IF EXISTS judging_rounds CASCADE;
DROP TABLE IF EXISTS teams CASCADE;
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS user_competitions CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS competitions CASCADE;

DROP FUNCTION IF EXISTS fn_judging_scores_enforce_integrity() CASCADE;
DROP FUNCTION IF EXISTS fn_judging_slots_sync_round_id() CASCADE;

DROP TYPE IF EXISTS judging_slot_status CASCADE;
DROP TYPE IF EXISTS judging_round_status CASCADE;
DROP TYPE IF EXISTS reset_token_purpose CASCADE;
DROP TYPE IF EXISTS competition_role CASCADE;
