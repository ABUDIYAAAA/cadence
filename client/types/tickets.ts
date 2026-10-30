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
  attendee: AttendeeDetails;
}

export interface DuplicateCheckInResponse {
  error: string;
  checked_in_at: string;
  checked_in_by_id?: string | null;
  checked_in_by_name?: string | null;
  attendee: AttendeeDetails;
}

export interface BulkSendTicketsRequest {
  user_ids?: string[];
  team_id?: string;
  role?: string;
  skip_checked_in?: boolean;
  exclude_domains?: string[];
}

export interface BulkSendTicketsResponse {
  total_processed: number;
  enqueued: number;
  skipped: number;
}

export interface SingleSendTicketResponse {
  message: string;
  user_id: string;
  ticket_code: string;
}

export interface MyTicketResponse {
  ticket_code: string;
  is_checked_in: boolean;
  checked_in_at?: string | null;
  qr_code_base64: string;
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
  hourly_arrivals: HourlyArrival[];
  team_completeness: TeamCompletenessSummary[];
}

export type ScanKind = "attendance" | "exit";

export interface ScanSession {
  id: string;
  name: string;
  kind: ScanKind;
  is_open: boolean;
  mark_team: boolean;
  scanned_count: number;
  created_at: string;
  closed_at?: string | null;
}

export interface CreateScanSessionRequest {
  name: string;
  kind: ScanKind;
  mark_team?: boolean;
}

export interface TeamScanResult {
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
  team_name?: string | null;
  is_external: boolean;
  scanned_at?: string | null;
}

export interface SessionReport {
  session: ScanSession;
  eligible: number;
  scanned: number;
  missing: SessionPerson[];
  recent: SessionPerson[];
}

export interface TicketSettings {
  internal_email_domains: string[];
  source: "database" | "environment";
}
