CREATE TABLE IF NOT EXISTS competition_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role event_role NOT NULL DEFAULT 'participant',
    is_banned BOOLEAN NOT NULL DEFAULT FALSE,
    banned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_competition_members_comp_user UNIQUE (competition_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_competition_members_comp_role ON competition_members(competition_id, role);
CREATE INDEX IF NOT EXISTS idx_competition_members_user ON competition_members(user_id);

-- Backfill existing competition team members so they appear in their competition roster.
INSERT INTO competition_members (competition_id, user_id, role)
SELECT DISTINCT tm.competition_id, tm.user_id, COALESCE(ue.role, 'participant'::event_role)
FROM team_members tm
JOIN competitions c ON c.id = tm.competition_id
LEFT JOIN user_events ue ON ue.event_id = c.event_id AND ue.user_id = tm.user_id
ON CONFLICT (competition_id, user_id) DO NOTHING;
