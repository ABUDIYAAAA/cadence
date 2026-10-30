"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ticketsService } from "@/services/tickets.service";
import {
  BulkSendTicketsRequest,
  CheckInRequest,
  CreateScanSessionRequest,
} from "@/types/tickets";

export const TICKETS_QUERY_KEY = ["tickets"] as const;

export function useMyTicket() {
  return useQuery({
    queryKey: [...TICKETS_QUERY_KEY, "me"],
    queryFn: () => ticketsService.getMyTicket(),
  });
}

export function useTicketAnalytics() {
  return useQuery({
    queryKey: [...TICKETS_QUERY_KEY, "analytics"],
    queryFn: () => ticketsService.getAnalytics(),
    refetchInterval: 1000 * 30, // poll every 30 seconds for live gate check-in counts
  });
}

export function useCheckIn() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CheckInRequest) => ticketsService.checkIn(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...TICKETS_QUERY_KEY, "analytics"],
      });
    },
  });
}

export function useBulkSendTickets() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: BulkSendTicketsRequest) =>
      ticketsService.bulkSend(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...TICKETS_QUERY_KEY, "analytics"],
      });
    },
  });
}

export function useSendSingleTicket() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (userId: string) => ticketsService.sendSingle(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...TICKETS_QUERY_KEY, "analytics"],
      });
    },
  });
}

export function useReleaseTickets() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: BulkSendTicketsRequest) => ticketsService.release(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "analytics"] });
    },
  });
}

export function useScanSessions() {
  return useQuery({
    queryKey: [...TICKETS_QUERY_KEY, "sessions"],
    queryFn: () => ticketsService.listSessions(),
    refetchInterval: 1000 * 15,
  });
}

export function useCreateScanSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateScanSessionRequest) => ticketsService.createSession(payload),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "sessions"] }),
  });
}

export function useSetScanSessionOpen() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, isOpen }: { id: string; isOpen: boolean }) =>
      ticketsService.setSessionOpen(id, isOpen),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "sessions"] }),
  });
}

export function useSessionScan() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionId, ticketCode }: { sessionId: string; ticketCode: string }) =>
      ticketsService.scanInSession(sessionId, ticketCode),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "sessions"] }),
  });
}

export function useSessionReport(id: string | null) {
  return useQuery({
    queryKey: [...TICKETS_QUERY_KEY, "sessions", "report", id],
    queryFn: () => ticketsService.getSessionReport(id as string),
    enabled: Boolean(id),
    refetchInterval: 1000 * 10,
  });
}

export function useTicketSettings() {
  return useQuery({
    queryKey: [...TICKETS_QUERY_KEY, "settings"],
    queryFn: () => ticketsService.getSettings(),
  });
}

export function useUpdateTicketSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domains: string[]) => ticketsService.updateSettings(domains),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "settings"] });
      queryClient.invalidateQueries({ queryKey: [...TICKETS_QUERY_KEY, "sessions"] });
    },
  });
}
