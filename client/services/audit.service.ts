import type { AxiosRequestConfig } from "axios";
import { apiClient } from "@/lib/api-client";
import { AuditLogFilter, PaginatedAuditLogsResponse } from "@/types/audit";

type PlatformRequestConfig = AxiosRequestConfig & { skipTenantScope?: boolean };

export const auditService = {
  async list(filters?: AuditLogFilter): Promise<PaginatedAuditLogsResponse> {
    const { data } = await apiClient.get<PaginatedAuditLogsResponse>("/audit-logs", {
      params: filters,
    });
    return data;
  },

  async listPlatform(
    filters?: AuditLogFilter
  ): Promise<PaginatedAuditLogsResponse> {
    const config: PlatformRequestConfig = {
      params: filters,
      skipTenantScope: true,
    };
    const { data } = await apiClient.get<PaginatedAuditLogsResponse>(
      "/audit-logs",
      config
    );
    return data;
  },

  async export(filters?: AuditLogFilter): Promise<Blob> {
    const { data } = await apiClient.get("/audit-logs/export", {
      params: filters,
      responseType: "blob",
    });
    return data;
  },

  async exportPlatform(filters?: AuditLogFilter): Promise<Blob> {
    const config: PlatformRequestConfig = {
      params: filters,
      responseType: "blob",
      skipTenantScope: true,
    };
    const { data } = await apiClient.get("/audit-logs/export", config);
    return data;
  },
};
