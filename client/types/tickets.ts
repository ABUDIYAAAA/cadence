export interface CheckInRequest {
  ticket_code: string;
}

export interface AttendeeDetails {
  id: string;
  name: string;
  email: string;
  role: string;
  phone_number?: string | null;
  team_id?: string | null;
  team_name?: string | null;
}

export interface CheckInResponse {
  success: boolean;
  message: string;
  ticket_code: string;
  checked_in_at: string;
  checked_in_by_id?: string;
  checked_in_by_email?: string;
  attendee: AttendeeDetails;
}

export interface DuplicateCheckInResponse {
  error: string;
  checked_in_at: string;
  checked_in_by_id?: string | null;
  checked_in_by_name?: string | null;
  attendee: AttendeeDetails;
}

export interface SendSingleTicketRequest {
  email_subject?: string;
  email_html?: string;
  force_resend?: boolean;
}

export interface BulkSendTicketsRequest {
  user_ids?: string[];
  team_id?: string;
  role?: string;
  skip_checked_in?: boolean;
  exclude_domains?: string[];
  email_subject?: string;
  email_html?: string;
  force_resend?: boolean;
}

export interface BulkSendTicketsResponse {
  total_processed: number;
  enqueued: number;
  already_mailed?: number;
  excluded_domain?: number;
  skipped: number;
}

export interface SingleSendTicketResponse {
  message: string;
  user_id: string;
  ticket_code: string;
  email_status?: string;
}

export interface AttendedSessionSummary {
  session_id: string;
  session_name: string;
  kind: ScanKind;
  scanned_at: string;
  scanned_by_id?: string | null;
  scanned_by_name?: string | null;
}

export interface MyTicketResponse {
  ticket_code: string;
  is_checked_in: boolean;
  checked_in_at?: string | null;
  email_status?: string;
  email_sent_at?: string | null;
  released_at?: string | null;
  qr_code_base64: string;
  attended_sessions?: AttendedSessionSummary[];
}

export interface HourlyArrival {
  hour: string;
  count: number;
}

export interface TeamCompletenessSummary {
  team_id: string;
  team_name: string;
  total_members: number;
  checked_in_members: number;
  status: "complete" | "partial" | "absent" | string;
}

export interface AnalyticsResponse {
  total_issued: number;
  checked_in_count: number;
  /** Percentage from 0 to 100 (not a 0-1 fraction). */
  attendance_rate: number;
  email_sent_count?: number;
  email_queued_count?: number;
  email_failed_count?: number;
  email_skipped_domain_count?: number;
  hourly_arrivals: HourlyArrival[];
  team_completeness: TeamCompletenessSummary[];
}

export type ScanKind = "check_in" | "attendance" | "exit";

export interface ScanSession {
  id: string;
  event_id?: string;
  competition_id?: string | null;
  competition_slug?: string | null;
  competition_name?: string | null;
  name: string;
  kind: ScanKind;
  is_default?: boolean;
  is_open: boolean;
  mark_team: boolean;
  checkin_code?: string | null;
  checkin_code_updated_at?: string | null;
  scanned_count: number;
  created_by_id?: string | null;
  created_by_name?: string | null;
  created_at: string;
  closed_at?: string | null;
}

export interface CreateScanSessionRequest {
  name: string;
  kind: ScanKind;
  mark_team?: boolean;
  competition_id?: string | null;
  generate_code?: boolean;
  checkin_code?: string;
}

export interface TeamScanResult {
  team_id?: string;
  team_name: string;
  marked: number;
  already_scanned: number;
  skipped: { name: string; reason: "not_checked_in" | "no_pass" | string }[];
}

export interface SessionScanResponse {
  success: boolean;
  message: string;
  session: ScanSession;
  scanned_at: string;
  scanned_by_id?: string;
  scanned_by_name?: string;
  scanned_by_email?: string;
  attendee: AttendeeDetails;
  team?: TeamScanResult;
}

export interface ReleaseTicketsResponse {
  total_processed: number;
  released: number;
  already_had: number;
  skipped: number;
}

export interface SessionPerson {
  user_id: string;
  name: string;
  email: string;
  phone_number?: string | null;
  team_id?: string | null;
  team_name?: string | null;
  is_leader?: boolean;
  is_external: boolean;
  scanned_at?: string | null;
  scanned_by_id?: string | null;
  scanned_by_name?: string | null;
  present: boolean;
}

export interface SessionReport {
  session: ScanSession;
  eligible: number;
  scanned: number;
  missing: SessionPerson[];
  recent: SessionPerson[];
  /** Everyone expected, present and absent. */
  people: SessionPerson[];
}

export interface TicketSettings {
  internal_email_domains: string[];
  excluded_domains?: string[];
  email_subject?: string;
  email_html?: string;
  tickets_released?: boolean;
  source: "database" | "environment";
}

export interface UpdateTicketSettingsRequest {
  internal_email_domains?: string[];
  excluded_domains?: string[];
  email_subject?: string;
  email_html?: string;
  tickets_released?: boolean;
}
