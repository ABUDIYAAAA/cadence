export type GlobalRole = "dev" | "user";

export type EventRole = "owner" | "admin" | "judge" | "volunteer" | "participant";

export type AssignableEventRole = "admin" | "judge" | "volunteer" | "participant";

export type UserRole = EventRole;

export interface UserEventRole {
  event_id: string;
  event_slug: string;
  event_name: string;
  role: EventRole;
}

export interface UserResponse {
  id: string;
  email: string;
  name: string;
  phone_number?: string | null;
  global_role?: GlobalRole;
  google_id?: string | null;
  avatar_url?: string | null;
  role: UserRole;
  team_id?: string | null;
  events?: UserEventRole[];
  created_at: string;
}

export interface AuthResponse {
  user: UserResponse;
  message: string;
}

export interface MessageResponse {
  message: string;
}
