"use client";

import { keepPreviousData, useMutation, useQuery } from "@tanstack/react-query";
import { useTenant } from "@/providers/tenant-provider";
import { auditService } from "@/services/audit.service";
import { AuditLogFilter } from "@/types/audit";

export const AUDIT_QUERY_KEY = ["audit-logs"] as const;

export function useAuditLogs(
  filters?: AuditLogFilter,
  options?: { enabled?: boolean }
) {
  const { eventSlug } = useTenant();
  return useQuery({
    queryKey: [...AUDIT_QUERY_KEY, "list", eventSlug, filters],
    queryFn: () => auditService.list(filters),
    enabled: options?.enabled ?? true,
    placeholderData: keepPreviousData,
  });
}

export function usePlatformAuditLogs(
  filters?: AuditLogFilter,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: [...AUDIT_QUERY_KEY, "platform-list", filters],
    queryFn: () => auditService.listPlatform(filters),
    enabled: options?.enabled ?? true,
    placeholderData: keepPreviousData,
  });
}

export function useExportAuditLogs() {
  return useMutation({
    mutationFn: (filters?: AuditLogFilter) => auditService.export(filters),
  });
}

export function useExportPlatformAuditLogs() {
  return useMutation({
    mutationFn: (filters?: AuditLogFilter) => auditService.exportPlatform(filters),
  });
}
