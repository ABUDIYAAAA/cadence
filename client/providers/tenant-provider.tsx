"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/providers/auth-provider";
import { useCompetitions } from "@/hooks/use-competitions";
import {
  clearTenantScope,
  getSavedCompetitionForEvent,
  getTenantScope,
  setTenantScope,
  subscribeTenantScope,
  TenantScope,
} from "@/lib/api-client";
import { UserEventRole, UserRole } from "@/types/auth";
import { Competition } from "@/types/competitions";

interface TenantContextType {
  eventSlug: string | null;
  competitionSlug: string | null;
  activeEvent: UserEventRole | null;
  events: UserEventRole[];
  activeCompetition: Competition | null;
  competitions: Competition[];
  isLoadingCompetitions: boolean;
  effectiveRole: UserRole | null;
  isDev: boolean;
  isOwner: boolean;
  isAdmin: boolean;
  isJudge: boolean;
  isVolunteer: boolean;
  isParticipant: boolean;
  selectEvent: (eventSlug: string, initialCompetitionSlug?: string | null) => void;
  selectCompetition: (competitionSlug: string | null) => void;
  clearEvent: () => void;
}

const TenantContext = createContext<TenantContextType | undefined>(undefined);

const SCOPED_QUERY_PREFIXES = [
  "teams",
  "judging",
  "tickets",
  "notifications",
  "audit-logs",
  "users",
] as const;

export function TenantProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const {
    user,
    isAuthenticated,
    effectiveRole,
    isDev,
    isOwner,
    isAdmin,
    isJudge,
    isVolunteer,
    isParticipant,
    isLoading: authLoading,
  } = useAuth();
  const [scope, setScopeState] = useState<TenantScope>(() => getTenantScope());

  useEffect(() => {
    return subscribeTenantScope((next) => {
      setScopeState(next);
    });
  }, []);

  const events = useMemo<UserEventRole[]>(
    () => user?.events ?? [],
    [user?.events]
  );

  // Verify the saved eventSlug still belongs to this user (unless dev)
  useEffect(() => {
    if (authLoading || !isAuthenticated || !user) return;
    if (!scope.eventSlug) return;
    if (isDev) return;

    const isEnrolled = events.some((e) => e.event_slug === scope.eventSlug);
    if (!isEnrolled) {
      clearTenantScope();
    }
  }, [authLoading, isAuthenticated, user, isDev, events, scope.eventSlug]);

  const activeEvent = useMemo<UserEventRole | null>(() => {
    if (!scope.eventSlug) return null;
    const found = events.find((e) => e.event_slug === scope.eventSlug);
    if (found) return found;
    if (isDev) {
      return {
        event_id: "",
        event_slug: scope.eventSlug,
        event_name: scope.eventSlug,
        role: "owner",
      };
    }
    return null;
  }, [scope.eventSlug, events, isDev]);

  const {
    data: competitions = [],
    isLoading: isLoadingCompetitions,
    isFetching: isFetchingCompetitions,
  } = useCompetitions(scope.eventSlug, {
    enabled: isAuthenticated && Boolean(scope.eventSlug),
  });

  const invalidateScopedQueries = useCallback(
    (onlyCompetitionScoped = false) => {
      const prefixes = onlyCompetitionScoped
        ? (["teams", "judging", "users"] as const)
        : SCOPED_QUERY_PREFIXES;
      prefixes.forEach((key) => {
        queryClient.invalidateQueries({ queryKey: [key] });
      });
    },
    [queryClient]
  );

  const selectEvent = useCallback(
    (nextEventSlug: string, initialCompetitionSlug?: string | null) => {
      const savedComp =
        initialCompetitionSlug !== undefined
          ? initialCompetitionSlug
          : getSavedCompetitionForEvent(nextEventSlug);
      setTenantScope({
        eventSlug: nextEventSlug,
        competitionSlug: savedComp ?? null,
      });
      invalidateScopedQueries(false);
    },
    [invalidateScopedQueries]
  );

  const selectCompetition = useCallback(
    (nextCompetitionSlug: string | null) => {
      setTenantScope({ competitionSlug: nextCompetitionSlug });
      invalidateScopedQueries(true);
    },
    [invalidateScopedQueries]
  );

  // Auto-select the saved or first competition when competitions load for the active event
  useEffect(() => {
    if (!scope.eventSlug || isLoadingCompetitions || isFetchingCompetitions) {
      return;
    }

    if (competitions.length === 0) {
      if (scope.competitionSlug !== null) {
        selectCompetition(null);
      }
      return;
    }

    const currentValid =
      scope.competitionSlug &&
      competitions.some((c) => c.slug === scope.competitionSlug);
    if (currentValid) return;

    const saved = getSavedCompetitionForEvent(scope.eventSlug);
    const savedValid = saved && competitions.some((c) => c.slug === saved);
    const nextSlug = savedValid ? saved : competitions[0].slug;

    selectCompetition(nextSlug);
  }, [
    scope.eventSlug,
    scope.competitionSlug,
    competitions,
    isLoadingCompetitions,
    isFetchingCompetitions,
    selectCompetition,
  ]);

  const activeCompetition = useMemo<Competition | null>(() => {
    if (!scope.competitionSlug) return null;
    return competitions.find((c) => c.slug === scope.competitionSlug) ?? null;
  }, [competitions, scope.competitionSlug]);

  const clearEvent = useCallback(() => {
    clearTenantScope();
    invalidateScopedQueries(false);
  }, [invalidateScopedQueries]);

  const value = useMemo<TenantContextType>(
    () => ({
      eventSlug: scope.eventSlug,
      competitionSlug: scope.competitionSlug,
      activeEvent,
      events,
      activeCompetition,
      competitions,
      isLoadingCompetitions,
      effectiveRole,
      isDev,
      isOwner,
      isAdmin,
      isJudge,
      isVolunteer,
      isParticipant,
      selectEvent,
      selectCompetition,
      clearEvent,
    }),
    [
      scope.eventSlug,
      scope.competitionSlug,
      activeEvent,
      events,
      activeCompetition,
      competitions,
      isLoadingCompetitions,
      effectiveRole,
      isDev,
      isOwner,
      isAdmin,
      isJudge,
      isVolunteer,
      isParticipant,
      selectEvent,
      selectCompetition,
      clearEvent,
    ]
  );

  return (
    <TenantContext.Provider value={value}>{children}</TenantContext.Provider>
  );
}

export function useTenant() {
  const context = useContext(TenantContext);
  if (!context) {
    throw new Error("useTenant must be used within a TenantProvider");
  }
  return context;
}
