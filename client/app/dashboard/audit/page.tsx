"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Download,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import { useAuditLogs, useExportAuditLogs } from "@/hooks/use-audit";
import { AuditLogFilter, AuditLogResponse } from "@/types/audit";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { Alert } from "@/components/ui/alert";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  AuditFilters,
  AuditTable,
  AuditDetailDialog,
} from "@/components/audit";

export default function AuditLogsPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAdmin, isLoading: authLoading } = useAuth();

  // Filters state
  const [filters, setFilters] = useState<AuditLogFilter>({
    page: 1,
    limit: 20,
    search: "",
    action: undefined,
    target_type: undefined,
    from_date: undefined,
    to_date: undefined,
  });

  // Query & Export
  const {
    data: auditResponse,
    isLoading: auditLoading,
    isFetching,
    refetch,
  } = useAuditLogs(filters);

  const exportMutation = useExportAuditLogs();

  // Dialog state
  const [selectedLog, setSelectedLog] = useState<AuditLogResponse | null>(null);

  // Guards
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace("/login");
    }
  }, [authLoading, user, router]);

  if (authLoading || !user) {
    return (
      <div className="p-6 font-mono">
        <SkeletonCard />
      </div>
    );
  }

  if (!isAdmin) {
    return (
      <div className="p-6 font-mono max-w-4xl mx-auto space-y-4">
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

  // Handlers
  const handleExportCsv = async () => {
    try {
      toast.info("Generating CSV export from audit stream...", "Export started");
      const blob = await exportMutation.mutateAsync(filters);

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

  return (
    <div className="space-y-6 font-sans">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
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
            Activity and security events across the platform.
          </p>
        </div>

        {/* Action Toolbar */}
        <div className="flex items-center gap-2">
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
            isLoading={exportMutation.isPending}
            className="h-9 px-4 text-xs gap-1.5 font-semibold"
          >
            <Download className="h-3.5 w-3.5" />
            <span>Export CSV</span>
          </Button>
        </div>
      </div>

      {/* Primary Surface: Filters + Table + Pagination */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        <AuditFilters
          filters={filters}
          onChange={(newFilters) => setFilters(newFilters)}
          onReset={() =>
            setFilters({
              page: 1,
              limit: filters.limit,
              search: "",
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
