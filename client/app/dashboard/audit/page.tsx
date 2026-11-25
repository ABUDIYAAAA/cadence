"use client";

import React, { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import {
  Download,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useAuditLogs,
  useExportAuditLogs,
  useExportPlatformAuditLogs,
  usePlatformAuditLogs,
} from "@/hooks/use-audit";
import { useEvents } from "@/hooks/use-events";
import { useCompetitions } from "@/hooks/use-competitions";
import { AuditLogFilter, AuditLogResponse } from "@/types/audit";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { Alert } from "@/components/ui/alert";
import { apiErrorMessage } from "@/lib/utils";
import { AdminTablePageSkeleton } from "@/components/ui/skeleton";
import {
  AuditFilters,
  AuditTable,
  AuditDetailDialog,
} from "@/components/audit";

export default function AuditLogsPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAdmin, isDev, isLoading: authLoading } = useAuth();
  const { eventSlug, activeEvent, competitions: tenantCompetitions } = useTenant();

  const [selectedMode, setSelectedMode] = useState<"event" | "platform">(() =>
    !eventSlug ? "platform" : "event"
  );
  const mode: "event" | "platform" =
    isDev && !eventSlug ? "platform" : isDev ? selectedMode : "event";

  // Filters state
  const [filters, setFilters] = useState<AuditLogFilter>({
    page: 1,
    limit: 20,
    search: "",
    event_id: undefined,
    competition_id: undefined,
    action: undefined,
    target_type: undefined,
    from_date: undefined,
    to_date: undefined,
  });

  // For platform dev mode: load all orgs/events and cascade competitions when an org is picked
  const { data: allEvents = [] } = useEvents({
    enabled: Boolean(user) && isDev && mode === "platform",
  });

  const selectedOrg = useMemo(() => {
    if (mode !== "platform" || !filters.event_id || filters.event_id === "platform") {
      return null;
    }
    return allEvents.find((ev) => ev.id === filters.event_id) ?? null;
  }, [mode, filters.event_id, allEvents]);

  const { data: selectedOrgCompetitions = [] } = useCompetitions(
    selectedOrg?.slug ?? null,
    { enabled: Boolean(selectedOrg?.slug) }
  );

  const visibleCompetitions =
    mode === "event" ? tenantCompetitions : selectedOrgCompetitions;

  // Query & Export
  const eventAuditQuery = useAuditLogs(filters, {
    enabled: Boolean(user) && isAdmin && mode === "event",
  });
  const platformAuditQuery = usePlatformAuditLogs(filters, {
    enabled: Boolean(user) && isDev && mode === "platform",
  });

  const activeQuery =
    mode === "platform" ? platformAuditQuery : eventAuditQuery;
  const auditResponse = activeQuery.data;
  const auditLoading = activeQuery.isLoading;
  const isFetching = activeQuery.isFetching;
  const auditError = activeQuery.isError;
  const auditErrorObj = activeQuery.error;
  const refetch = activeQuery.refetch;

  const exportEventMutation = useExportAuditLogs();
  const exportPlatformMutation = useExportPlatformAuditLogs();
  const isExporting =
    exportEventMutation.isPending || exportPlatformMutation.isPending;

  // Dialog state
  const [selectedLog, setSelectedLog] = useState<AuditLogResponse | null>(null);

  // Guards
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace("/login");
    }
  }, [authLoading, user, router]);

  if (authLoading || !user) {
    return <AdminTablePageSkeleton />;
  }

  if (!isAdmin) {
    return (
      <div className="font-mono space-y-4">
        <Alert variant="error" title="Access restricted">
          <div className="flex items-center gap-2">
            <ShieldAlert className="h-5 w-5" />
            <span>
              You do not have administrative privileges to inspect the security audit trail.
            </span>
          </div>
          <div className="mt-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => router.push("/dashboard")}
            >
              Return to dashboard
            </Button>
          </div>
        </Alert>
      </div>
    );
  }

  const handleModeSwitch = (nextMode: "event" | "platform") => {
    setSelectedMode(nextMode);
    setFilters((prev) => ({
      ...prev,
      page: 1,
      event_id: undefined,
      competition_id: undefined,
    }));
  };

  // Handlers
  const handleExportCsv = async () => {
    try {
      toast.info("Generating CSV export from audit stream...", "Export started");
      const blob =
        mode === "platform"
          ? await exportPlatformMutation.mutateAsync(filters)
          : await exportEventMutation.mutateAsync(filters);

      // Create download anchor
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
      a.download = `audit_logs_${timestamp}.csv`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);

      toast.success(
        "Audit log CSV downloaded successfully.",
        "Export complete"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to export audit logs";
      toast.error(errorMsg, "Export failed");
    }
  };

  const displayEventName = activeEvent?.event_name || eventSlug || "this event";

  return (
    <div className="space-y-6 font-sans">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Audit log
            </h1>
            {auditResponse?.total_count !== undefined && (
              <Badge variant="subtle" size="sm">
                {auditResponse.total_count} {auditResponse.total_count === 1 ? "event" : "events"}
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            {mode === "platform"
              ? "Activity and security events across the platform and all organizations."
              : `Activity and security events scoped to ${displayEventName}.`}
          </p>
        </div>

        {/* Action Toolbar */}
        <div className="flex flex-wrap items-center gap-2">
          {isDev && Boolean(eventSlug) && (
            <div className="inline-flex rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-0.5 text-xs">
              <button
                type="button"
                onClick={() => handleModeSwitch("event")}
                className={`px-2.5 py-1.5 rounded-[8px] font-medium transition-colors cursor-pointer ${
                  mode === "event"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                }`}
              >
                This Event
              </button>
              <button
                type="button"
                onClick={() => handleModeSwitch("platform")}
                className={`px-2.5 py-1.5 rounded-[8px] font-medium transition-colors cursor-pointer ${
                  mode === "platform"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                }`}
              >
                All Events
              </button>
            </div>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh audit logs"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          <Button
            variant="primary"
            size="sm"
            onClick={handleExportCsv}
            isLoading={isExporting}
            className="h-9 px-4 text-xs gap-1.5 font-semibold"
          >
            <Download className="h-3.5 w-3.5" />
            <span>Export CSV</span>
          </Button>
        </div>
      </div>

      {auditError && (
        <Alert variant="error" title="Could not load audit log">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <span>{apiErrorMessage(auditErrorObj, "Failed to fetch audit records from the server.")}</span>
            <Button variant="outline" size="sm" onClick={() => refetch()} className="h-8 px-3 text-xs shrink-0">
              Try again
            </Button>
          </div>
        </Alert>
      )}

      {/* Primary Surface: Filters + Table + Pagination */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <AuditFilters
          filters={filters}
          events={mode === "platform" ? allEvents : []}
          competitions={visibleCompetitions}
          onChange={(newFilters) => setFilters(newFilters)}
          onReset={() =>
            setFilters({
              page: 1,
              limit: filters.limit,
              search: "",
              event_id: undefined,
              competition_id: undefined,
              action: undefined,
              target_type: undefined,
              from_date: undefined,
              to_date: undefined,
            })
          }
        />

        <AuditTable
          logs={auditResponse?.data ?? []}
          isLoading={auditLoading}
          onView={(log) => setSelectedLog(log)}
        />

        {auditResponse && (
          <Pagination
            page={auditResponse.page}
            totalPages={auditResponse.total_pages}
            totalCount={auditResponse.total_count}
            limit={auditResponse.limit}
            onPageChange={(page) => setFilters((prev) => ({ ...prev, page }))}
            onLimitChange={(limit) =>
              setFilters((prev) => ({ ...prev, limit, page: 1 }))
            }
          />
        )}
      </div>

      {/* Event Details Inspector Modal */}
      <AuditDetailDialog
        log={selectedLog}
        open={Boolean(selectedLog)}
        onClose={() => setSelectedLog(null)}
      />
    </div>
  );
}
