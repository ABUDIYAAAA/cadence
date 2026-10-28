import { UserRole } from "@/types/auth";

export function getDashboardRoute(role?: UserRole | string | null): string {
  switch (role) {
    case "admin":
      return "/dashboard/admin";
    case "judge":
      return "/dashboard/judge";
    case "volunteer":
      return "/dashboard/volunteer";
    case "participant":
      return "/dashboard/participant";
    default:
      return "/dashboard";
  }
}
