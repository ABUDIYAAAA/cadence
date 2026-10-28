export type UserRole = "admin" | "judge" | "volunteer" | "participant";

export interface UserResponse {
  id: string;
  email: string;
  name: string;
  phone_number?: string | null;
  role: UserRole;
  team_id?: string | null;
  is_password_configured: boolean;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface SetupPasswordRequest {
  token: string;
  password: string;
  confirm_password: string;
}

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  password: string;
  confirm_password: string;
}

export interface AuthResponse {
  user: UserResponse;
  message: string;
}

export interface MessageResponse {
  message: string;
}
