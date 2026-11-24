"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { getTenantScope } from "@/lib/api-client";
import { auditService } from "@/services/audit.service";
import { AuditLogFilter } from "@/types/audit";

export const AUDIT_QUERY_KEY = ["audit-logs"] as const;

export function useAuditLogs(filters?: AuditLogFilter) {
  const { eventSlug } = getTenantScope();
  return useQuery({
    queryKey: [...AUDIT_QUERY_KEY, "list", eventSlug, filters],
    queryFn: () => auditService.list(filters),
  });
}

export function useExportAuditLogs() {
  return useMutation({
    mutationFn: (filters?: AuditLogFilter) => auditService.export(filters),
  });
}
