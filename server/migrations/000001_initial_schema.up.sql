-- ==============================================================================
-- CADENCE V2 CONSOLIDATED CORE SCHEMA
-- Multi-Tenant Competition Management Platform
-- ==============================================================================

-- 1. Extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 2. Enumerated Types
DO $$ BEGIN
    CREATE TYPE competition_role AS ENUM (
        'owner',
        'admin',
        'volunteer',
        'judge',
        'participant'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE reset_token_purpose AS ENUM (
        'initial_setup',
        'forgot_password'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE judging_round_status AS ENUM (
        'draft',
        'active',
        'completed'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE judging_slot_status AS ENUM (
        'scheduled',
        'queued',
        'in_progress',
        'completed',
        'skipped',
        'no_show'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

-- 3. Competitions (Tenant Root)
CREATE TABLE IF NOT EXISTS competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_competitions_slug ON competitions(slug);

-- 4. Users (Global Account Identity)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NULL,
    name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(32) NULL,
    is_password_configured BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    token_version INT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- 5. User Competitions (Scoped Membership & Single-Ownership)
CREATE TABLE IF NOT EXISTS user_competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role competition_role NOT NULL DEFAULT 'participant',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (competition_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_user_competitions_user_id ON user_competitions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_competitions_comp_role ON user_competitions(competition_id, role);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_competitions_unique_owner ON user_competitions(user_id) WHERE role = 'owner';

-- 6. Refresh Tokens & Password Reset Tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    family_id UUID NOT NULL DEFAULT gen_random_uuid(),
    parent_id UUID NULL REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    device_id VARCHAR(255) NULL,
    user_agent TEXT NULL,
    ip_address VARCHAR(45) NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    purpose reset_token_purpose NOT NULL DEFAULT 'forgot_password',
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);

-- 7. Teams (Scoped to Competition)
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    join_code VARCHAR(8) NOT NULL,
    leader_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    is_frozen BOOLEAN NOT NULL DEFAULT FALSE,
    max_size INT NULL CHECK (max_size IS NULL OR max_size >= 1),
    project_name VARCHAR(150) NULL,
    submission_links JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (competition_id, name),
    UNIQUE (competition_id, join_code)
);
CREATE INDEX IF NOT EXISTS idx_teams_competition_id ON teams(competition_id);
CREATE INDEX IF NOT EXISTS idx_teams_leader_id ON teams(leader_id);
CREATE INDEX IF NOT EXISTS idx_teams_join_code ON teams(join_code);

-- User team association (supports legacy and fast user-team dereference)
ALTER TABLE users ADD COLUMN IF NOT EXISTS team_id UUID NULL REFERENCES teams(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_team_id ON users(team_id);

-- 8. Judging Module (Scoped to Competition)
CREATE TABLE IF NOT EXISTS judging_rounds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    status judging_round_status NOT NULL DEFAULT 'draft',
    buffer_size INT NOT NULL DEFAULT 2,
    slot_duration_mins INT NOT NULL DEFAULT 15,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_judging_rounds_competition_id ON judging_rounds(competition_id);

CREATE TABLE IF NOT EXISTS judging_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    team_id UUID NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    description TEXT NULL,
    min_score INT NOT NULL DEFAULT 0,
    max_score INT NOT NULL DEFAULT 10,
    weight NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    order_index INT NOT NULL DEFAULT 0,
    awarded_score NUMERIC(6,2) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_judging_criteria_id_round UNIQUE (id, round_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_criteria_round_id ON judging_criteria(round_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_judging_criteria_round_team ON judging_criteria (round_id, team_id) WHERE team_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS judging_panels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    room VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_judging_panels_id_round UNIQUE (id, round_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_panels_round_id ON judging_panels(round_id);

CREATE TABLE IF NOT EXISTS judging_panel_judges (
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (panel_id, judge_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_panel_judges_judge_id ON judging_panel_judges(judge_id);

CREATE TABLE IF NOT EXISTS judging_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    sequence_order INT NOT NULL,
    status judging_slot_status NOT NULL DEFAULT 'scheduled',
    estimated_start_time TIMESTAMPTZ NULL,
    actual_start_time TIMESTAMPTZ NULL,
    actual_end_time TIMESTAMPTZ NULL,
    pitch_ended_at TIMESTAMPTZ NULL,
    paused_at TIMESTAMPTZ NULL,
    paused_total_ms BIGINT NOT NULL DEFAULT 0,
    extra_secs INT NOT NULL DEFAULT 0,
    duration_mins INT NULL CHECK (duration_mins IS NULL OR duration_mins > 0),
    scheduled_start_time TIMESTAMPTZ NULL,
    timer_state VARCHAR(32) NOT NULL DEFAULT 'idle',
    timer_elapsed_seconds INT NOT NULL DEFAULT 0,
    timer_last_started_at TIMESTAMPTZ NULL,
    extended_seconds INT NOT NULL DEFAULT 0,
    submission_links JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (panel_id, team_id),
    UNIQUE (panel_id, sequence_order)
);
CREATE INDEX IF NOT EXISTS idx_judging_slots_panel_id ON judging_slots(panel_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_team_id ON judging_slots(team_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_round_id ON judging_slots(round_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_status ON judging_slots(status);

CREATE TABLE IF NOT EXISTS judging_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    criterion_id UUID NOT NULL REFERENCES judging_criteria(id) ON DELETE CASCADE,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    score NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    notes TEXT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    locked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (slot_id, judge_id, criterion_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_scores_slot_id ON judging_scores(slot_id);
CREATE INDEX IF NOT EXISTS idx_judging_scores_judge_id ON judging_scores(judge_id);
CREATE INDEX IF NOT EXISTS idx_judging_scores_round_id ON judging_scores(round_id);

CREATE TABLE IF NOT EXISTS judging_round_teams (
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (round_id, team_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_round_teams_team ON judging_round_teams(team_id);

CREATE TABLE IF NOT EXISTS judging_score_events (
    id BIGSERIAL PRIMARY KEY,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    criterion_id UUID NULL REFERENCES judging_criteria(id) ON DELETE CASCADE,
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('score_set', 'score_changed', 'locked', 'unlocked')),
    old_score NUMERIC(5,2) NULL,
    new_score NUMERIC(5,2) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_judging_score_events_round ON judging_score_events(round_id, created_at);
CREATE INDEX IF NOT EXISTS idx_judging_score_events_judge ON judging_score_events(judge_id, created_at);
CREATE INDEX IF NOT EXISTS idx_judging_score_events_slot ON judging_score_events(slot_id);

CREATE TABLE IF NOT EXISTS judging_judge_metrics (
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    slot_started_at TIMESTAMPTZ NULL,
    first_score_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    grading_ms BIGINT NULL,
    slot_ms BIGINT NULL,
    edit_count INT NOT NULL DEFAULT 0,
    criteria_count INT NOT NULL DEFAULT 0,
    score_total NUMERIC(8,2) NULL,
    score_avg NUMERIC(6,3) NULL,
    score_pct NUMERIC(6,5) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (slot_id, judge_id)
);
CREATE INDEX IF NOT EXISTS idx_judging_judge_metrics_round ON judging_judge_metrics(round_id);
CREATE INDEX IF NOT EXISTS idx_judging_judge_metrics_judge ON judging_judge_metrics(judge_id);

-- Judging Triggers & Functions
CREATE OR REPLACE FUNCTION fn_judging_slots_sync_round_id()
RETURNS TRIGGER AS $$
DECLARE
    v_panel_round_id UUID;
BEGIN
    SELECT round_id INTO v_panel_round_id
    FROM judging_panels
    WHERE id = NEW.panel_id;

    IF v_panel_round_id IS NULL THEN
        RAISE EXCEPTION 'judging panel % does not exist', NEW.panel_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF NEW.round_id IS NULL THEN
        NEW.round_id := v_panel_round_id;
    ELSIF NEW.round_id <> v_panel_round_id THEN
        RAISE EXCEPTION 'slot round_id % does not match panel round_id %', NEW.round_id, v_panel_round_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_judging_slots_sync_round_id ON judging_slots;
CREATE TRIGGER trg_judging_slots_sync_round_id
BEFORE INSERT OR UPDATE OF panel_id, round_id ON judging_slots
FOR EACH ROW EXECUTE FUNCTION fn_judging_slots_sync_round_id();

CREATE OR REPLACE FUNCTION fn_judging_scores_enforce_integrity()
RETURNS TRIGGER AS $$
DECLARE
    v_slot_round_id UUID;
    v_crit_round_id UUID;
    v_min_score INT;
    v_max_score INT;
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.is_locked = TRUE THEN
        IF NEW.score IS DISTINCT FROM OLD.score OR
           NEW.notes IS DISTINCT FROM OLD.notes OR
           NEW.criterion_id IS DISTINCT FROM OLD.criterion_id OR
           NEW.slot_id IS DISTINCT FROM OLD.slot_id OR
           NEW.judge_id IS DISTINCT FROM OLD.judge_id THEN
            RAISE EXCEPTION 'scorecard is locked and cannot be edited'
                USING ERRCODE = 'check_violation';
        END IF;
        IF NEW.is_locked = FALSE AND COALESCE(current_setting('cadence.allow_unlock', true), '') <> 'on' THEN
            RAISE EXCEPTION 'scorecard is locked and cannot be edited'
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;

    SELECT round_id INTO v_slot_round_id
    FROM judging_slots
    WHERE id = NEW.slot_id;

    IF v_slot_round_id IS NULL THEN
        RAISE EXCEPTION 'judging slot % does not exist', NEW.slot_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF NEW.round_id IS NULL THEN
        NEW.round_id := v_slot_round_id;
    ELSIF NEW.round_id <> v_slot_round_id THEN
        RAISE EXCEPTION 'score round_id % does not match slot round_id %', NEW.round_id, v_slot_round_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    SELECT round_id, min_score, max_score
    INTO v_crit_round_id, v_min_score, v_max_score
    FROM judging_criteria
    WHERE id = NEW.criterion_id;

    IF v_crit_round_id IS NULL THEN
        RAISE EXCEPTION 'judging criterion % does not exist', NEW.criterion_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF v_crit_round_id <> v_slot_round_id THEN
        RAISE EXCEPTION 'criterion round_id % does not match slot round_id %', v_crit_round_id, v_slot_round_id
            USING ERRCODE = 'check_violation';
    END IF;

    IF NEW.score < v_min_score OR NEW.score > v_max_score THEN
        RAISE EXCEPTION 'score % is outside allowed bounds [%, %] for criterion %', NEW.score, v_min_score, v_max_score, NEW.criterion_id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_judging_scores_enforce_integrity ON judging_scores;
CREATE TRIGGER trg_judging_scores_enforce_integrity
BEFORE INSERT OR UPDATE ON judging_scores
FOR EACH ROW EXECUTE FUNCTION fn_judging_scores_enforce_integrity();

-- 9. Tickets & Attendance Module (Scoped to Competition)
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ticket_code VARCHAR(100) NOT NULL UNIQUE,
    is_checked_in BOOLEAN NOT NULL DEFAULT FALSE,
    checked_in_at TIMESTAMPTZ NULL,
    checked_in_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    email_sent_at TIMESTAMPTZ NULL,
    released_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (competition_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_tickets_competition_id ON tickets(competition_id);
CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_ticket_code ON tickets(ticket_code);
CREATE INDEX IF NOT EXISTS idx_tickets_checked_in ON tickets(is_checked_in);

CREATE TABLE IF NOT EXISTS scan_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('attendance', 'exit')),
    is_open BOOLEAN NOT NULL DEFAULT TRUE,
    mark_team BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_scan_sessions_competition_id ON scan_sessions(competition_id);

CREATE TABLE IF NOT EXISTS scan_events (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES scan_sessions(id) ON DELETE CASCADE,
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    scanned_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (session_id, ticket_id)
);
CREATE INDEX IF NOT EXISTS idx_scan_events_ticket ON scan_events(ticket_id);
CREATE INDEX IF NOT EXISTS idx_scan_events_session_time ON scan_events(session_id, scanned_at DESC);

CREATE TABLE IF NOT EXISTS app_settings (
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value JSONB NOT NULL,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (competition_id, key)
);
CREATE INDEX IF NOT EXISTS idx_app_settings_competition ON app_settings(competition_id);

-- 10. Notifications Module (Scoped to Competition)
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    user_agent TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_push_subs_user_id ON push_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_push_subs_endpoint ON push_subscriptions(endpoint);

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    action_url TEXT NULL,
    icon_url TEXT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id VARCHAR(255) NULL,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notifications_competition_id ON notifications(competition_id);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);

CREATE TABLE IF NOT EXISTS notification_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (notification_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_notif_recipients_user_id ON notification_recipients(user_id);
CREATE INDEX IF NOT EXISTS idx_notif_recipients_is_read ON notification_recipients(is_read);

-- 11. Audit Module (Scoped to Competition or Platform)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NULL REFERENCES competitions(id) ON DELETE SET NULL,
    actor_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    actor_email VARCHAR(255) NULL,
    actor_role VARCHAR(50) NULL,
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id VARCHAR(255) NULL,
    description TEXT NOT NULL,
    metadata JSONB NULL,
    device_id VARCHAR(255) NULL,
    user_agent TEXT NULL,
    ip_address VARCHAR(45) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_competition_id ON audit_logs(competition_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

-- 12. Default Initial Competition Seed
INSERT INTO competitions (id, slug, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'devclub-hack', 'DevClub Hackathon', 'Default initial competition')
ON CONFLICT (id) DO NOTHING;
