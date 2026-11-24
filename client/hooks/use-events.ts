"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { eventsService } from "@/services/events.service";
import { AUTH_QUERY_KEY } from "@/hooks/use-auth";
import { USERS_QUERY_KEY } from "@/hooks/use-users";
import {
  BanMemberRequest,
  CreateEventRequest,
  TransferOwnershipRequest,
  UpdateEventRequest,
  UpdateMemberRoleRequest,
} from "@/types/events";

export const EVENTS_QUERY_KEY = ["events"] as const;

export function useEvents(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...EVENTS_QUERY_KEY, "list"],
    queryFn: () => eventsService.listEvents(),
    enabled: options?.enabled ?? true,
  });
}

export function useEventDetail(
  eventSlug: string | null | undefined,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: [...EVENTS_QUERY_KEY, "detail", eventSlug],
    queryFn: () => eventsService.getEvent(eventSlug!),
    enabled: Boolean(eventSlug) && (options?.enabled ?? true),
  });
}

export function useCreateEvent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateEventRequest) => eventsService.createEvent(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EVENTS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
    },
  });
}

export function useUpdateEvent(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateEventRequest) =>
      eventsService.updateEvent(eventSlug, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EVENTS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
    },
  });
}

export function useDeleteEvent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (eventSlug: string) => eventsService.deleteEvent(eventSlug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EVENTS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
    },
  });
}

export function useTransferEventOwnership(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: TransferOwnershipRequest) =>
      eventsService.transferOwnership(eventSlug, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EVENTS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useUpdateEventMemberRole(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      userId,
      payload,
    }: {
      userId: string;
      payload: UpdateMemberRoleRequest;
    }) => eventsService.updateMemberRole(eventSlug, userId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useRemoveEventMember(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => eventsService.removeMember(eventSlug, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useBanEventMember(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      userId,
      payload,
    }: {
      userId: string;
      payload?: BanMemberRequest;
    }) => eventsService.banMember(eventSlug, userId, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}

export function useUnbanEventMember(eventSlug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => eventsService.unbanMember(eventSlug, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: USERS_QUERY_KEY });
    },
  });
}
