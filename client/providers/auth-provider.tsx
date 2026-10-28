"use client";

import React, { createContext, useContext, useCallback, useMemo } from "react";
import { useRouter } from "next/navigation";
import { UserResponse, UserRole } from "@/types/auth";
import { useCurrentUser, useLogout } from "@/hooks/use-auth";
import { unsubscribePush } from "@/lib/push-notifications";

interface AuthContextType {
  user: UserResponse | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  role: UserRole | null;
  isAdmin: boolean;
  isJudge: boolean;
  isVolunteer: boolean;
  isParticipant: boolean;
  logout: () => Promise<void>;
  refetchUser: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { data: userData, isLoading, isError, refetch } = useCurrentUser();
  const logoutMutation = useLogout();

  // If there's an auth error (e.g. 401 unauthenticated), user is null
  const user = isError ? null : userData ?? null;
  const role = user?.role ?? null;
  const isAdmin = role === "admin";
  const isJudge = role === "judge";
  const isVolunteer = role === "volunteer";
  const isParticipant = role === "participant";

  const handleLogout = useCallback(async () => {
    // 1. Attempt push notification cleanup with strict 400ms timeout
    // Must NEVER block or hang the user logout
    try {
      const pushCleanupPromise = unsubscribePush();
      const timeoutPromise = new Promise((resolve) => setTimeout(resolve, 400));
      await Promise.race([pushCleanupPromise, timeoutPromise]);
    } catch (pushErr) {
      console.warn("Push subscription cleanup error during logout:", pushErr);
    }

    try {
      // 2. Clear any session-scoped notification prompt flags
      sessionStorage.removeItem("cadence_push_prompt_dismissed");
    } catch {}

    try {
      // 3. Invalidate auth cookies and revoke refresh token on backend
      await logoutMutation.mutateAsync();
      router.push("/login");
    } catch (err) {
      console.error("Logout error", err);
      router.push("/login");
    }
  }, [logoutMutation, router]);

  const refetchUser = useCallback(() => {
    refetch();
  }, [refetch]);

  const value = useMemo(
    () => ({
      user: user ?? null,
      isLoading,
      isAuthenticated: Boolean(user),
      role,
      isAdmin,
      isJudge,
      isVolunteer,
      isParticipant,
      logout: handleLogout,
      refetchUser,
    }),
    [user, isLoading, role, isAdmin, isJudge, isVolunteer, isParticipant, handleLogout, refetchUser]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
