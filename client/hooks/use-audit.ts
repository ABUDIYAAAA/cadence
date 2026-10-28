"use client";

import { useQuery } from "@tanstack/react-query";
import { auditService } from "@/services/audit.service";
import { AuditLogFilter } from "@/types/audit";

export const AUDIT_QUERY_KEY = ["audit-logs"] as const;

export function useAuditLogs(filters?: AuditLogFilter) {
  return useQuery({
    queryKey: [...AUDIT_QUERY_KEY, "list", filters],
    queryFn: () => auditService.list(filters),
  });
}
