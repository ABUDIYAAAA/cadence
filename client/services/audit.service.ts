import { apiClient } from "@/lib/api-client";
import { AuditLogFilter, PaginatedAuditLogsResponse } from "@/types/audit";

export const auditService = {
  async list(filters?: AuditLogFilter): Promise<PaginatedAuditLogsResponse> {
    const { data } = await apiClient.get<PaginatedAuditLogsResponse>("/audit-logs", {
      params: filters,
    });
    return data;
  },

  async export(filters?: AuditLogFilter): Promise<Blob> {
    const { data } = await apiClient.get("/audit-logs/export", {
      params: filters,
      responseType: "blob",
    });
    return data;
  },
};
