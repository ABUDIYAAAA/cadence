export interface CreateCriteriaRequest {
  name: string;
  description?: string;
  min_score?: number;
  max_score: number;
  weight: number;
  order_index?: number;
}

export interface CreateRoundRequest {
  name: string;
  buffer_size?: number;
  slot_duration_mins?: number;
  criteria: CreateCriteriaRequest[];
}

export interface CreatePanelRequest {
  round_id: string;
  name: string;
  room: string;
  judge_ids: string[];
}

export interface SlotAssignment {
  team_id: string;
  sequence_order: number;
  submission_links?: Record<string, string>;
}

export interface ConfigureSlotsRequest {
  slots: SlotAssignment[];
}

export interface UpdateSlotLinksRequest {
  submission_links: Record<string, string>;
}

export interface CriterionScoreInput {
  criterion_id: string;
  score: number;
  notes?: string;
}

export interface SubmitScoresRequest {
  scores: CriterionScoreInput[];
}

export interface CriterionOutput {
  id: string;
  name: string;
  description: string;
  min_score: number;
  max_score: number;
  weight: number;
  order_index: number;
}

export interface CriterionScoreOutput {
  criterion_id: string;
  criterion_name: string;
  min_score: number;
  max_score: number;
  weight: number;
  score?: number | null;
  normalized_score?: number | null;
  notes?: string | null;
}

export interface JudgeSlotSummary {
  slot_id: string;
  team_id: string;
  team_name: string;
  sequence_order: number;
  status: "scheduled" | "queued" | "in_progress" | "completed" | string;
  estimated_start_time?: string | null;
  actual_start_time?: string | null;
  actual_end_time?: string | null;
  is_my_scorecard_locked: boolean;
  all_panel_judges_locked: boolean;
}

export interface JudgeSlotDetails {
  slot_id: string;
  panel_id: string;
  panel_name: string;
  room: string;
  team_id: string;
  team_name: string;
  sequence_order: number;
  status: string;
  slot_duration_mins: number;
  slot_duration_seconds: number;
  actual_start_time?: string | null;
  target_end_time?: string | null;
  submission_links: Record<string, string>;
  criteria_scores: CriterionScoreOutput[];
  is_my_scorecard_locked: boolean;
  all_panel_judges_locked: boolean;
}

export interface JudgePanelSummary {
  panel_id: string;
  panel_name: string;
  room: string;
  round_id: string;
  round_name: string;
  round_status: string;
  slot_duration_mins: number;
  buffer_size: number;
  criteria: CriterionOutput[];
  total_slots: number;
  completed_slots: number;
  current_slot?: JudgeSlotSummary | null;
  slots: JudgeSlotSummary[];
}

export interface TeamStatusResponse {
  has_active_round: boolean;
  round_name?: string;
  panel_name?: string;
  room?: string;
  status?: "scheduled" | "queued" | "in_progress" | "completed" | string;
  queue_position?: number;
  estimated_start_time?: string | null;
  estimated_duration_mins?: number;
  instructions?: string;
  submission_links?: Record<string, string>;
}

export interface TeamSlotSummary {
  slot_id: string;
  team_id: string;
  team_name: string;
  sequence_order: number;
  status: string;
  estimated_start_time?: string | null;
  actual_start_time?: string | null;
  submission_links?: Record<string, string>;
}

export interface VolunteerPanelSchedule {
  panel_id: string;
  panel_name: string;
  room: string;
  round_id: string;
  round_name: string;
  slot_duration_mins: number;
  buffer_size: number;
  active_team?: TeamSlotSummary | null;
  queued_teams: TeamSlotSummary[];
  upcoming_teams: TeamSlotSummary[];
  completed_count: number;
  total_count: number;
}

export interface AdminLeaderboardEntry {
  rank: number;
  team_id: string;
  team_name: string;
  panel_name: string;
  room: string;
  slot_status: string;
  criteria_averages: Record<string, number>;
  criteria_weighted_scores?: Record<string, number>;
  total_weighted_score: number;
  max_possible_score: number;
  average_raw_score: number;
  average_score: number;
  judges_evaluated_count: number;
  total_judges_on_panel: number;
  all_panel_judges_locked: boolean;
}

export interface AdminLeaderboardResponse {
  round_id: string;
  round_name: string;
  status: string;
  criteria: CriterionOutput[];
  leaderboard: AdminLeaderboardEntry[];
  total_teams: number;
  graded_teams: number;
  max_possible_score: number;
  average_score: number;
}

export interface RoundSummary {
  id: string;
  name: string;
  status: "draft" | "active" | "completed" | string;
  buffer_size: number;
  slot_duration_mins: number;
  created_at: string;
  panel_count?: number;
  slot_count?: number;
}

export interface RoundDetail {
  id: string;
  name: string;
  status: string;
  buffer_size: number;
  slot_duration_mins: number;
  criteria: CriterionOutput[];
}

export interface CreateRoundResponse {
  message: string;
  round_id: string;
  id?: string;
}

export interface CreatePanelResponse {
  message: string;
  panel_id: string;
  id?: string;
}
