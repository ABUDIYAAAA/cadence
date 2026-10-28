"use client";

import React, { createContext, useContext, useEffect, useMemo } from "react";
import { UserResponse, UserRole } from "@/types/auth";
import { useCurrentUser, useLogout } from "@/hooks/use-auth";

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
  const { data: user, isLoading, refetch } = useCurrentUser();
  const logoutMutation = useLogout();

  const role = user?.role ?? null;
  const isAdmin = role === "admin";
  const isJudge = role === "judge";
  const isVolunteer = role === "volunteer";
  const isParticipant = role === "participant";

  const handleLogout = async () => {
    try {
      await logoutMutation.mutateAsync();
      window.location.href = "/login";
    } catch (err) {
      console.error("Logout error", err);
    }
  };

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
      refetchUser: () => {
        refetch();
      },
    }),
    [user, isLoading, role, isAdmin, isJudge, isVolunteer, isParticipant]
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
