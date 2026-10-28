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

export interface VolunteerLeaderboard {
  volunteer_id: string;
  volunteer_name: string;
  volunteer_email: string;
  count: number;
}

export interface AnalyticsResponse {
  total_issued: number;
  checked_in_count: number;
  attendance_rate: number;
  hourly_arrivals: HourlyArrival[];
  team_completeness: TeamCompletenessSummary[];
  volunteer_leaderboard: VolunteerLeaderboard[];
}
