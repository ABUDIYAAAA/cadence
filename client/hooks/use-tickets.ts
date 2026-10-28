"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ticketsService } from "@/services/tickets.service";
import { BulkSendTicketsRequest, CheckInRequest } from "@/types/tickets";

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
