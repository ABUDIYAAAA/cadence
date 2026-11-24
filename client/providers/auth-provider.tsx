"use client";

import React, {
  createContext,
  useContext,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import { UserResponse, UserRole } from "@/types/auth";
import { useCurrentUser, useLogout } from "@/hooks/use-auth";
import {
  clearTenantScope,
  getTenantScope,
  subscribeTenantScope,
} from "@/lib/api-client";
import { resolveEffectivePermissions } from "@/lib/auth-routes";
import { unsubscribePush } from "@/lib/push-notifications";

interface AuthContextType {
  user: UserResponse | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  role: UserRole | null;
  effectiveRole: UserRole | null;
  isDev: boolean;
  isOwner: boolean;
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
  const [activeEventSlug, setActiveEventSlug] = useState<string | null>(
    () => getTenantScope().eventSlug
  );

  useEffect(() => {
    return subscribeTenantScope((scope) => {
      setActiveEventSlug(scope.eventSlug);
    });
  }, []);

  // If there's an auth error (e.g. 401 unauthenticated), user is null
  const rawUser = isError ? null : userData ?? null;
  const permissions = useMemo(
    () => resolveEffectivePermissions(rawUser, activeEventSlug),
    [rawUser, activeEventSlug]
  );

  const role: UserRole | null =
    permissions.effectiveRole ?? rawUser?.role ?? null;

  const user = useMemo<UserResponse | null>(() => {
    if (!rawUser) return null;
    return {
      ...rawUser,
      role: (role ?? "participant") as UserRole,
    };
  }, [rawUser, role]);

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
      // 2. Clear any session-scoped notification prompt flags and active tenant scope
      sessionStorage.removeItem("cadence_push_prompt_dismissed");
      clearTenantScope();
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
      effectiveRole: permissions.effectiveRole,
      isDev: permissions.isDev,
      isOwner: permissions.isOwner,
      isAdmin: permissions.isAdmin,
      isJudge: permissions.isJudge,
      isVolunteer: permissions.isVolunteer,
      isParticipant: permissions.isParticipant,
      logout: handleLogout,
      refetchUser,
    }),
    [user, isLoading, role, permissions, handleLogout, refetchUser]
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
