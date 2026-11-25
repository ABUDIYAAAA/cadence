"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { buildScopedQueryKey, isScopeReady } from "@/lib/api-client";
import { useTenant } from "@/providers/tenant-provider";
import { ticketsService } from "@/services/tickets.service";
import {
  BulkSendTicketsRequest,
  CheckInRequest,
  CreateScanSessionRequest,
  SendSingleTicketRequest,
  UpdateTicketSettingsRequest,
} from "@/types/tickets";

export const TICKETS_QUERY_KEY = ["tickets"] as const;

function useTicketsScope() {
  const { eventSlug, competitionSlug } = useTenant();
  const scope = { eventSlug, competitionSlug };
  const ready = isScopeReady("event", scope);
  const key = (...parts: unknown[]) =>
    buildScopedQueryKey("tickets", "event", scope, ...parts);
  return { scope, ready, key };
}

export function useMyTicket(options?: { enabled?: boolean }) {
  const { ready, key } = useTicketsScope();
  return useQuery({
    queryKey: key("me"),
    queryFn: () => ticketsService.getMyTicket(),
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useTicketAnalytics(options?: { enabled?: boolean }) {
  const { ready, key } = useTicketsScope();
  return useQuery({
    queryKey: key("analytics"),
    queryFn: () => ticketsService.getAnalytics(),
    enabled: ready && (options?.enabled ?? true),
    refetchInterval: 1000 * 30, // poll every 30 seconds for live gate check-in counts
  });
}

export function useCheckIn() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();

  return useMutation({
    mutationFn: (payload: CheckInRequest) => ticketsService.checkIn(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("analytics"),
      });
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      });
    },
  });
}

export function useCheckInByCode() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();

  return useMutation({
    mutationFn: (code: string) => ticketsService.checkInByCode(code),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("me"),
      });
      queryClient.invalidateQueries({
        queryKey: key("analytics"),
      });
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      });
    },
  });
}

export function useBulkSendTickets() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();

  return useMutation({
    mutationFn: (payload: BulkSendTicketsRequest) =>
      ticketsService.bulkSend(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("analytics"),
      });
      queryClient.invalidateQueries({
        queryKey: ["users"],
      });
    },
  });
}

export function useSendSingleTicket() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();

  return useMutation({
    mutationFn: (
      arg: string | { userId: string; payload?: SendSingleTicketRequest }
    ) =>
      typeof arg === "string"
        ? ticketsService.sendSingle(arg)
        : ticketsService.sendSingle(arg.userId, arg.payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("analytics"),
      });
      queryClient.invalidateQueries({
        queryKey: ["users"],
      });
    },
  });
}

export function useReleaseTickets() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: (payload: BulkSendTicketsRequest) =>
      ticketsService.release(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("analytics"),
      });
      queryClient.invalidateQueries({
        queryKey: ["users"],
      });
    },
  });
}

export function useScanSessions(options?: { enabled?: boolean }) {
  const { ready, key } = useTicketsScope();
  return useQuery({
    queryKey: key("sessions"),
    queryFn: () => ticketsService.listSessions(),
    enabled: ready && (options?.enabled ?? true),
    refetchInterval: 1000 * 15,
  });
}

export function useCreateScanSession() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: (payload: CreateScanSessionRequest) =>
      ticketsService.createSession(payload),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useSetScanSessionOpen() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: ({ id, isOpen }: { id: string; isOpen: boolean }) =>
      ticketsService.setSessionOpen(id, isOpen),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useGenerateSessionCode() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: ({
      sessionId,
      code,
    }: {
      sessionId: string;
      code?: string;
    }) => ticketsService.generateSessionCode(sessionId, code),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useClearSessionCode() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: (sessionId: string) =>
      ticketsService.clearSessionCode(sessionId),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useGenerateDefaultCheckInCode() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: (code?: string) =>
      ticketsService.generateDefaultCheckInCode(code),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useClearDefaultCheckInCode() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: () => ticketsService.clearDefaultCheckInCode(),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useSessionScan() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: ({
      sessionId,
      ticketCode,
    }: {
      sessionId: string;
      ticketCode: string;
    }) => ticketsService.scanInSession(sessionId, ticketCode),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      }),
  });
}

export function useSessionReport(id: string | null) {
  const { ready, key } = useTicketsScope();
  return useQuery({
    queryKey: key("sessions", "report", id),
    queryFn: () => ticketsService.getSessionReport(id as string),
    enabled: ready && Boolean(id),
    refetchInterval: 1000 * 10,
  });
}

export function useTicketSettings(options?: { enabled?: boolean }) {
  const { ready, key } = useTicketsScope();
  return useQuery({
    queryKey: key("settings"),
    queryFn: () => ticketsService.getSettings(),
    enabled: ready && (options?.enabled ?? true),
  });
}

export function useUpdateTicketSettings() {
  const queryClient = useQueryClient();
  const { key } = useTicketsScope();
  return useMutation({
    mutationFn: (payload: UpdateTicketSettingsRequest) =>
      ticketsService.updateSettings(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: key("settings"),
      });
      queryClient.invalidateQueries({
        queryKey: key("sessions"),
      });
    },
  });
}
