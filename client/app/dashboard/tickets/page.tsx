"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Ticket,
  QrCode,
  Scan,
  RefreshCw,
  Users,
  BarChart3,
  CheckCircle2,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useBulkSendTickets,
  useSendSingleTicket,
  useTicketAnalytics,
} from "@/hooks/use-tickets";
import { useUsers } from "@/hooks/use-users";
import { UserDetailResponse, UserFilter } from "@/types/users";
import { BulkSendTicketsRequest } from "@/types/tickets";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/pagination";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import {
  TicketScanner,
  TicketBulkSendDialog,
  TicketUserTable,
  MyTicketView,
} from "@/components/tickets";

export default function TicketsPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAdmin, isVolunteer, isParticipant, isLoading: authLoading } = useAuth();

  // Active view tab for Admin
  const [adminTab, setAdminTab] = useState<"users" | "scanner" | "analytics">("users");

  // Filters for User Directory
  const [userFilters, setUserFilters] = useState<UserFilter>({
    page: 1,
    limit: 20,
    search: "",
    role: undefined,
  });

  // Selected users for bulk send
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([]);
  const [isBulkSendOpen, setIsBulkSendOpen] = useState(false);
  const [actionInProgressId, setActionInProgressId] = useState<string | null>(null);

  // Queries & Mutations
  const {
    data: usersResponse,
    isLoading: usersLoading,
    isFetching: usersFetching,
    refetch: refetchUsers,
  } = useUsers(userFilters);

  const { data: analyticsData } = useTicketAnalytics();

  const bulkSendMutation = useBulkSendTickets();
  const sendSingleMutation = useSendSingleTicket();

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

  // 1. PARTICIPANT VIEW: Show their personal QR pass if issued
  if (isParticipant && !isAdmin && !isVolunteer) {
    return (
      <div className="p-4 sm:p-6 lg:p-8 space-y-6 max-w-4xl mx-auto font-sans">
        <div className="pb-4 border-b border-[var(--card-line)]">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Digital event pass
          </h1>
          <p className="text-xs text-[var(--ink-secondary)] mt-1">
            Your admission ticket and QR verification code for registration check-in.
          </p>
        </div>

        <MyTicketView />
      </div>
    );
  }

  // Multi-select handlers
  const handleToggleSelect = (userId: string) => {
    setSelectedUserIds((prev) =>
      prev.includes(userId) ? prev.filter((id) => id !== userId) : [...prev, userId]
    );
  };

  const handleSelectAllPage = () => {
    const pageIds = (usersResponse?.data || []).map((u) => u.id);
    setSelectedUserIds((prev) => Array.from(new Set([...prev, ...pageIds])));
  };

  const handleClearSelection = () => {
    setSelectedUserIds([]);
  };

  // Bulk send handler
  const handleBulkSendTickets = async (req: BulkSendTicketsRequest) => {
    try {
      const res = await bulkSendMutation.mutateAsync(req);
      toast.success(
        `Dispatched ${res.enqueued} ticket emails. (${res.skipped} skipped)`,
        "Tickets enqueued"
      );
      setSelectedUserIds([]);
      return res;
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to dispatch bulk tickets";
      toast.error(errorMsg, "Dispatch failed");
      throw err;
    }
  };

  // Single send handler
  const handleSendSingleTicket = async (targetUser: UserDetailResponse) => {
    setActionInProgressId(targetUser.id);
    try {
      await sendSingleMutation.mutateAsync(targetUser.id);
      toast.success(
        `QR entry pass generated and emailed to ${targetUser.email}`,
        "QR pass dispatched"
      );
    } catch (err: unknown) {
      const errorMsg =
        err && typeof err === "object" && "error" in err
          ? (err as { error: string }).error
          : "Failed to issue ticket";
      toast.error(errorMsg, "Error");
    } finally {
      setActionInProgressId(null);
    }
  };

  const selectedUsers = (usersResponse?.data || []).filter((u) =>
    selectedUserIds.includes(u.id)
  );

  return (
    <div className="space-y-6 font-sans">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Tickets & check-in
            </h1>
            {analyticsData && (
              <Badge variant="subtle" size="sm">
                {analyticsData.checked_in_count} / {analyticsData.total_issued} checked in (
                {Math.round(analyticsData.attendance_rate * 100)}%)
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Issue passes, scan badges at the entrance, and track attendance.
          </p>
        </div>

        {/* Action Toolbar */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Admin Tab Switcher */}
          {isAdmin && (
            <div className="inline-flex rounded-lg border border-[var(--card-line)] p-1 bg-[var(--card-solid-subtle)] gap-1">
              <button
                type="button"
                onClick={() => setAdminTab("users")}
                className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
                  adminTab === "users"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                }`}
              >
                Attendees
              </button>
              <button
                type="button"
                onClick={() => setAdminTab("scanner")}
                className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
                  adminTab === "scanner"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                }`}
              >
                Scanner
              </button>
              <button
                type="button"
                onClick={() => setAdminTab("analytics")}
                className={`px-3 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer ${
                  adminTab === "analytics"
                    ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                    : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                }`}
              >
                Analytics
              </button>
            </div>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => refetchUsers()}
            disabled={usersFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh attendee data"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${usersFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {/* Admin Bulk Send Action */}
          {isAdmin && adminTab === "users" && selectedUserIds.length > 0 && (
            <Button
              variant="primary"
              size="sm"
              onClick={() => setIsBulkSendOpen(true)}
              className="h-9 px-4 text-xs gap-1.5"
            >
              <QrCode className="h-3.5 w-3.5" />
              <span>Send tickets ({selectedUserIds.length})</span>
            </Button>
          )}
        </div>
      </div>

      {/* 2. VOLUNTEER VIEW: Show Scanner + Attendee Directory */}
      {isVolunteer && !isAdmin && (
        <div className="space-y-6">
          <TicketScanner />

          <div className="space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--card-line)]">
              <h3 className="font-semibold text-sm text-[var(--ink-primary)]">
                Attendee roster lookup
              </h3>
            </div>

            {/* Quick search */}
            <div className="max-w-md">
              <Input
                placeholder="Search attendee by name or email..."
                value={userFilters.search || ""}
                onChange={(e) =>
                  setUserFilters((prev) => ({ ...prev, search: e.target.value, page: 1 }))
                }
                className="text-xs h-9 bg-[var(--card-solid)] border-[var(--card-line)]"
              />
            </div>

            <TicketUserTable
              users={usersResponse?.data || []}
              isLoading={usersLoading}
              selectedIds={[]}
              onToggleSelect={() => {}}
              onSelectAll={() => {}}
              onClearSelection={() => {}}
              onSendSingle={() => {}}
              canSendTickets={false}
            />

            {usersResponse && (
              <Pagination
                page={usersResponse.page}
                totalPages={usersResponse.total_pages}
                totalCount={usersResponse.total_count}
                limit={usersResponse.limit}
                onPageChange={(page) => setUserFilters((prev) => ({ ...prev, page }))}
                onLimitChange={(limit) =>
                  setUserFilters((prev) => ({ ...prev, limit, page: 1 }))
                }
              />
            )}
          </div>
        </div>
      )}

      {/* 3. ADMIN VIEW: Tabbed sections */}
      {isAdmin && (
        <>
          {adminTab === "scanner" && (
            <div className="space-y-6 max-w-4xl mx-auto">
              <TicketScanner />
            </div>
          )}

          {adminTab === "analytics" && analyticsData && (
            <div className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="rounded-[12px] border border-[var(--card-line)] p-4 bg-[var(--card-solid)]">
                  <div className="text-xs text-[var(--ink-secondary)]">
                    Total passes issued
                  </div>
                  <div className="text-2xl font-bold text-[var(--ink-primary)] mt-1">
                    {analyticsData.total_issued}
                  </div>
                </div>
                <div className="rounded-[12px] border border-[var(--card-line)] p-4 bg-[var(--card-solid)]">
                  <div className="text-xs text-[var(--ink-secondary)]">
                    Gate check-ins
                  </div>
                  <div className="text-2xl font-bold text-[var(--ink-primary)] mt-1">
                    {analyticsData.checked_in_count}
                  </div>
                </div>
                <div className="rounded-[12px] border border-[var(--card-line)] p-4 bg-[var(--card-solid)]">
                  <div className="text-xs text-[var(--ink-secondary)]">
                    Attendance rate
                  </div>
                  <div className="text-2xl font-bold text-[var(--ink-primary)] mt-1">
                    {Math.round(analyticsData.attendance_rate * 100)}%
                  </div>
                </div>
              </div>

              {/* Volunteer Leaderboard */}
              {analyticsData.volunteer_leaderboard.length > 0 && (
                <div className="rounded-[16px] border border-[var(--card-line)] p-5 bg-[var(--card-solid)] space-y-3">
                  <h4 className="font-semibold text-sm text-[var(--ink-primary)]">
                    Volunteer gate scan leaderboard
                  </h4>
                  <div className="divide-y divide-[var(--card-line)] border border-[var(--card-line)] rounded-[8px] overflow-hidden">
                    {analyticsData.volunteer_leaderboard.map((v, i) => (
                      <div key={v.volunteer_id} className="flex justify-between items-center p-3 text-xs bg-[var(--card-solid)]">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-[var(--ink-tertiary)]">#{i + 1}</span>
                          <span className="font-semibold text-[var(--ink-primary)]">{v.volunteer_name}</span>
                          <span className="text-[10px] text-[var(--ink-secondary)] font-mono">({v.volunteer_email})</span>
                        </div>
                        <Badge variant="subtle" size="sm">
                          {v.count} scans
                        </Badge>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {adminTab === "users" && (
            <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
              {/* Filter bar */}
              <div className="flex flex-col sm:flex-row gap-3 items-stretch sm:items-center justify-between">
                <div className="flex-1 max-w-md">
                  <Input
                    placeholder="Search attendees by name or email..."
                    value={userFilters.search || ""}
                    onChange={(e) =>
                      setUserFilters((prev) => ({ ...prev, search: e.target.value, page: 1 }))
                    }
                    className="text-xs h-9"
                  />
                </div>

                <div className="w-48">
                  <Select
                    value={userFilters.role || ""}
                    onChange={(e) =>
                      setUserFilters((prev) => ({ ...prev, role: e.target.value || undefined, page: 1 }))
                    }
                    className="text-xs h-9"
                  >
                    <option value="">All roles</option>
                    <option value="participant">Participants</option>
                    <option value="judge">Judges</option>
                    <option value="volunteer">Volunteers</option>
                    <option value="admin">Admins</option>
                  </Select>
                </div>
              </div>

              {/* Selection banner */}
              {selectedUserIds.length > 0 && (
                <div className="rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-3 px-4 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs">
                  <div className="flex items-center gap-2 text-[var(--ink-primary)]">
                    <CheckCircle2 className="h-4 w-4 text-[var(--color-blue)] shrink-0" />
                    <span>
                      <strong>{selectedUserIds.length}</strong> attendee
                      {selectedUserIds.length === 1 ? "" : "s"} selected
                    </span>
                  </div>

                  <div className="flex items-center gap-2">
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => setIsBulkSendOpen(true)}
                      className="h-8 text-xs gap-1.5 font-semibold"
                    >
                      <QrCode className="h-3.5 w-3.5" />
                      <span>Send passes</span>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleClearSelection}
                      className="h-8 text-xs"
                    >
                      Clear selection
                    </Button>
                  </div>
                </div>
              )}

              {/* Attendee table with multi-select */}
              <TicketUserTable
                users={usersResponse?.data || []}
                isLoading={usersLoading}
                selectedIds={selectedUserIds}
                onToggleSelect={handleToggleSelect}
                onSelectAll={handleSelectAllPage}
                onClearSelection={handleClearSelection}
                onSendSingle={handleSendSingleTicket}
                canSendTickets={true}
                actionInProgressId={actionInProgressId}
              />

              {/* Pagination */}
              {usersResponse && (
                <Pagination
                  page={usersResponse.page}
                  totalPages={usersResponse.total_pages}
                  totalCount={usersResponse.total_count}
                  limit={usersResponse.limit}
                  onPageChange={(page) => setUserFilters((prev) => ({ ...prev, page }))}
                  onLimitChange={(limit) =>
                    setUserFilters((prev) => ({ ...prev, limit, page: 1 }))
                  }
                />
              )}
            </div>
          )}
        </>
      )}

      {/* Bulk Send QR Dialog with Domain Filtering */}
      <TicketBulkSendDialog
        open={isBulkSendOpen}
        onClose={() => setIsBulkSendOpen(false)}
        selectedUsers={selectedUsers}
        onSubmit={handleBulkSendTickets}
        isLoading={bulkSendMutation.isPending}
      />
    </div>
  );
}
