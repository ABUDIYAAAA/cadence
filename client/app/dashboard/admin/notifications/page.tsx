"use client";

import React, { useState, useMemo } from "react";
import {
  ExternalLink,
  RefreshCw,
  Search,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useBroadcastNotification,
  useAdminNotificationHistory,
} from "@/hooks/use-notifications";
import { useTeams } from "@/hooks/use-teams";
import { useUsers } from "@/hooks/use-users";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Alert } from "@/components/ui/alert";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SkeletonCard } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ApiError } from "@/types/api";

export default function AdminNotificationsPage() {
  const { user, isAdmin, isLoading: authLoading } = useAuth();
  const toast = useToast();

  const [page, setPage] = useState(1);
  const limit = 15;

  // Form State
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [targetType, setTargetType] = useState<"broadcast" | "role" | "team" | "user">("broadcast");
  const [targetId, setTargetId] = useState("");
  const [roleSelection, setRoleSelection] = useState("participant");
  const [actionUrl, setActionUrl] = useState("");
  const [iconUrl, setIconUrl] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  // Manual ID toggle states
  const [isManualTeamId, setIsManualTeamId] = useState(false);
  const [isManualUserId, setIsManualUserId] = useState(false);
  const [userSearchQuery, setUserSearchQuery] = useState("");

  // Queries
  const broadcastMutation = useBroadcastNotification();
  const {
    data: historyData,
    isLoading: historyLoading,
    refetch: refetchHistory,
    isRefetching: historyRefetching,
  } = useAdminNotificationHistory(page, limit);

  // Fetch Teams for dropdown (only when team target is active or preloaded)
  const { data: teamsData, isLoading: teamsLoading } = useTeams({ limit: 100 });
  const teams = useMemo(() => teamsData?.data || [], [teamsData?.data]);

  // Fetch Users for dropdown (with optional search filter)
  const { data: usersData, isLoading: usersLoading } = useUsers({
    limit: 100,
    search: userSearchQuery.trim() || undefined,
  });
  const users = useMemo(() => usersData?.data || [], [usersData?.data]);

  // Currently selected team object
  const selectedTeam = useMemo(
    () => teams.find((t) => t.id === targetId),
    [teams, targetId]
  );

  // Currently selected user object
  const selectedUser = useMemo(
    () => users.find((u) => u.id === targetId),
    [users, targetId]
  );

  if (authLoading) {
    return <SkeletonCard />;
  }

  if (!isAdmin) {
    return (
      <Alert variant="error" title="Access restricted">
        You need administrator permissions to send broadcasts.
      </Alert>
    );
  }

  const effectiveTargetId =
    targetType === "role"
      ? roleSelection
      : targetType === "broadcast"
      ? undefined
      : targetId.trim() || undefined;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    if (!title.trim()) {
      setFormError("Title is required.");
      return;
    }
    if (!body.trim()) {
      setFormError("Message body is required.");
      return;
    }
    if ((targetType === "team" || targetType === "user") && !targetId.trim()) {
      setFormError(`Please select a ${targetType} from the list.`);
      return;
    }

    try {
      const resp = await broadcastMutation.mutateAsync({
        title: title.trim(),
        body: body.trim(),
        target_type: targetType,
        target_id: effectiveTargetId,
        action_url: actionUrl.trim() || undefined,
        icon_url: iconUrl.trim() || undefined,
      });

      toast.success(
        `Sent to ${resp.recipients_count} recipient(s). ${resp.enqueued_pushes} push notification(s) enqueued.`,
        "Notification sent"
      );

      // Reset form fields
      setTitle("");
      setBody("");
      setActionUrl("");
      setIconUrl("");
      setTargetId("");
      refetchHistory();
    } catch (err: unknown) {
      const apiErr = err as ApiError;
      const errorMsg =
        apiErr?.error || (err as Error)?.message || "Failed to send notification.";
      setFormError(errorMsg);
      toast.error(errorMsg, "Delivery failed");
    }
  };

  return (
    <div className="space-y-6 font-sans">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Broadcasts & alerts
          </h1>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Send web push notifications and in-app alerts to attendees, squads, or staff.
          </p>
        </div>

        <Badge variant="emerald" dot size="sm">
          Service active
        </Badge>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="compose">
        <TabsList>
          <TabsTrigger value="compose">
            Compose
          </TabsTrigger>
          <TabsTrigger value="history">
            History ({historyData?.total_count ?? 0})
          </TabsTrigger>
        </TabsList>

        {/* TAB 1: COMPOSE & DISPATCH */}
        <TabsContent value="compose">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
            {/* Form Column */}
            <div className="lg:col-span-7">
              <Card>
                <CardHeader>
                  <CardTitle>Compose notification</CardTitle>
                  <CardDescription>
                    Select recipients and write your announcement.
                  </CardDescription>
                </CardHeader>

                <form onSubmit={handleSubmit}>
                  <CardContent className="space-y-5">
                    {formError && (
                      <Alert variant="error" title="Please check the form">
                        {formError}
                      </Alert>
                    )}

                    {/* Target Audience Segmented Buttons */}
                    <div className="space-y-2">
                      <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
                        Recipients
                      </label>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-1 p-1 rounded-[8px] bg-[var(--card-solid-subtle)]">
                        {[
                          { id: "broadcast", label: "Everyone" },
                          { id: "role", label: "By role" },
                          { id: "team", label: "By team" },
                          { id: "user", label: "Specific user" },
                        ].map((item) => (
                          <button
                            key={item.id}
                            type="button"
                            onClick={() => {
                              setTargetType(
                                item.id as "broadcast" | "role" | "team" | "user"
                              );
                              setTargetId("");
                            }}
                            className={`px-2.5 py-1.5 text-xs rounded-[6px] transition-colors cursor-pointer text-center ${
                              targetType === item.id
                                ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs border border-[var(--card-line)] font-semibold"
                                : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] font-medium"
                            }`}
                          >
                            {item.label}
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* TARGET TYPE: ROLE */}
                    {targetType === "role" && (
                      <div className="space-y-1.5">
                        <Select
                          label="Select role"
                          value={roleSelection}
                          onChange={(e) => setRoleSelection(e.target.value)}
                        >
                          <option value="participant">Participant (hackers and attendees)</option>
                          <option value="judge">Judge (evaluators)</option>
                          <option value="volunteer">Volunteer (staff and coordinators)</option>
                          <option value="admin">Administrator</option>
                        </Select>
                        <p className="text-[11px] text-[var(--ink-secondary)]">
                          Delivered to active accounts with the <strong>{roleSelection}</strong> role.
                        </p>
                      </div>
                    )}

                    {/* TARGET TYPE: TEAM DROPDOWN */}
                    {targetType === "team" && (
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between">
                          <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
                            Squad
                          </label>
                          <button
                            type="button"
                            onClick={() => setIsManualTeamId(!isManualTeamId)}
                            className="text-[11px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] underline cursor-pointer"
                          >
                            {isManualTeamId ? "Select from list" : "Enter UUID manually"}
                          </button>
                        </div>

                        {isManualTeamId ? (
                          <Input
                            placeholder="Paste squad UUID..."
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            helperText="Target a specific squad UUID"
                          />
                        ) : (
                          <Select
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            disabled={teamsLoading}
                          >
                            <option value="">
                              {teamsLoading ? "Loading squads..." : "— Choose a squad —"}
                            </option>
                            {teams.map((t) => (
                              <option key={t.id} value={t.id}>
                                {t.name} ({t.member_count} member{t.member_count !== 1 ? "s" : ""})
                                {t.leader_name ? ` · Lead: ${t.leader_name}` : ""}
                              </option>
                            ))}
                          </Select>
                        )}

                        {selectedTeam && (
                          <p className="text-[11px] text-[var(--ink-secondary)]">
                            Squad lead: <span className="text-[var(--ink-primary)] font-medium">{selectedTeam.leader_name || "Unassigned"}</span> ({selectedTeam.member_count} members)
                          </p>
                        )}
                      </div>
                    )}

                    {/* TARGET TYPE: SINGLE USER DROPDOWN + SEARCH */}
                    {targetType === "user" && (
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between">
                          <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
                            Account
                          </label>
                          <button
                            type="button"
                            onClick={() => setIsManualUserId(!isManualUserId)}
                            className="text-[11px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] underline cursor-pointer"
                          >
                            {isManualUserId ? "Select from list" : "Enter UUID manually"}
                          </button>
                        </div>

                        {isManualUserId ? (
                          <Input
                            placeholder="Paste user UUID..."
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            helperText="Target a specific user UUID"
                          />
                        ) : (
                          <div className="space-y-2">
                            <div className="relative">
                              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
                              <input
                                type="text"
                                placeholder="Filter users by name or email..."
                                value={userSearchQuery}
                                onChange={(e) => setUserSearchQuery(e.target.value)}
                                className="w-full h-8 pl-8 pr-3 text-xs rounded-[6px] border border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-primary)] focus:outline-none focus:border-[var(--color-blue)]"
                              />
                            </div>

                            <Select
                              value={targetId}
                              onChange={(e) => setTargetId(e.target.value)}
                              disabled={usersLoading}
                            >
                              <option value="">
                                {usersLoading
                                  ? "Loading accounts..."
                                  : users.length === 0
                                  ? "No accounts match search"
                                  : "— Select an account —"}
                              </option>
                              {users.map((u) => (
                                <option key={u.id} value={u.id}>
                                  {u.name} — {u.email} ({u.role})
                                </option>
                              ))}
                            </Select>
                          </div>
                        )}

                        {selectedUser && (
                          <p className="text-[11px] text-[var(--ink-secondary)]">
                            Recipient: <span className="text-[var(--ink-primary)] font-medium">{selectedUser.name}</span> ({selectedUser.email}) · <span className="capitalize">{selectedUser.role}</span>
                          </p>
                        )}
                      </div>
                    )}

                    {/* Notification Title */}
                    <Input
                      label="Title"
                      required
                      placeholder="e.g. Round 1 judging starts in 10 minutes"
                      value={title}
                      maxLength={255}
                      onChange={(e) => setTitle(e.target.value)}
                      helperText={`${title.length}/255 characters`}
                    />

                    {/* Notification Body */}
                    <Textarea
                      label="Message"
                      required
                      rows={4}
                      placeholder="Enter announcement details, room locations, or instructions..."
                      value={body}
                      onChange={(e) => setBody(e.target.value)}
                    />

                    {/* Action link & Icon */}
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <Input
                        label="Action link (optional)"
                        placeholder="/dashboard/judging"
                        value={actionUrl}
                        onChange={(e) => setActionUrl(e.target.value)}
                        helperText="Opened when clicked"
                      />

                      <Input
                        label="Custom icon URL (optional)"
                        placeholder="/favicon.ico"
                        value={iconUrl}
                        onChange={(e) => setIconUrl(e.target.value)}
                        helperText="Optional push icon image"
                      />
                    </div>
                  </CardContent>

                  <CardFooter className="flex flex-col sm:flex-row justify-between items-center gap-3 pt-3 border-t border-[var(--card-line)]">
                    <span className="text-xs text-[var(--ink-secondary)]">
                      Queued and delivered via web push.
                    </span>

                    <Button
                      type="submit"
                      variant="primary"
                      size="md"
                      isLoading={broadcastMutation.isPending}
                      className="w-full sm:w-auto px-6 font-semibold"
                    >
                      <span>Send notification</span>
                    </Button>
                  </CardFooter>
                </form>
              </Card>
            </div>

            {/* Preview Column */}
            <div className="lg:col-span-5 space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>Preview</CardTitle>
                  <CardDescription>
                    Push notification appearance on subscriber devices.
                  </CardDescription>
                </CardHeader>

                <CardContent className="space-y-4">
                  {/* Push Mockup */}
                  <div className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-4 text-[var(--ink-primary)] space-y-2.5">
                    <div className="flex items-center justify-between pb-2 border-b border-[var(--card-line)]">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-semibold tracking-tight text-[var(--ink-primary)]">
                          Cadence
                          <span className="ml-1 font-medium text-[var(--ink-tertiary)]">by DevClub</span>
                        </span>
                        <span className="text-[10px] text-[var(--ink-tertiary)]">now</span>
                      </div>
                      <span className="text-[10px] font-medium text-[var(--ink-tertiary)]">
                        Push
                      </span>
                    </div>

                    <div>
                      <h4 className="text-xs font-semibold text-[var(--ink-primary)] break-words">
                        {title.trim() || "Notification title"}
                      </h4>
                      <p className="text-xs text-[var(--ink-secondary)] mt-1 leading-relaxed break-words">
                        {body.trim() ||
                          "Message details will appear here when delivered to user devices."}
                      </p>
                    </div>

                    {actionUrl && (
                      <div className="pt-2 border-t border-[var(--card-line)] flex items-center gap-1 text-[11px] font-medium text-[var(--color-blue)]">
                        <span>Open: {actionUrl}</span>
                        <ExternalLink className="h-3 w-3" />
                      </div>
                    )}
                  </div>

                  {/* Summary Box */}
                  <div className="space-y-2 text-xs pt-1">
                    <div className="flex justify-between items-center py-1 border-b border-[var(--card-line)]">
                      <span className="text-[var(--ink-secondary)]">Recipients</span>
                      <span className="font-medium text-[var(--ink-primary)]">
                        {targetType === "broadcast"
                          ? "Everyone"
                          : targetType === "role"
                          ? `Role: ${roleSelection}`
                          : targetType === "team"
                          ? `Squad: ${selectedTeam?.name || targetId || "Not selected"}`
                          : `User: ${selectedUser?.name || targetId || "Not selected"}`}
                      </span>
                    </div>
                    <div className="flex justify-between items-center py-1">
                      <span className="text-[var(--ink-secondary)]">Sender</span>
                      <span className="font-mono text-[var(--ink-secondary)]">{user?.email}</span>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        </TabsContent>

        {/* TAB 2: SENT NOTIFICATIONS */}
        <TabsContent value="history">
          <Card>
            <CardHeader>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div>
                  <CardTitle>Sent notifications</CardTitle>
                  <CardDescription>
                    Delivery counts and read rates for past broadcasts.
                  </CardDescription>
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => refetchHistory()}
                  isLoading={historyRefetching}
                  className="h-8 gap-1.5 text-xs"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  <span>Refresh</span>
                </Button>
              </div>
            </CardHeader>

            <CardContent>
              {historyLoading ? (
                <div className="p-8 text-center text-xs text-[var(--ink-secondary)]">
                  Loading history...
                </div>
              ) : !historyData?.items || historyData.items.length === 0 ? (
                <EmptyState
                  title="No notifications sent"
                  description="Use the compose tab to send your first broadcast or announcement."
                />
              ) : (
                <div className="space-y-4">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Time</TableHead>
                        <TableHead>Audience</TableHead>
                        <TableHead>Message</TableHead>
                        <TableHead>Sender</TableHead>
                        <TableHead className="text-right">Read rate</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {historyData.items.map((item) => {
                        const readPercentage =
                           item.recipients_count > 0
                            ? Math.round(
                                (item.read_count / item.recipients_count) * 100
                              )
                            : 0;

                        return (
                          <TableRow key={item.id}>
                            <TableCell className="text-xs whitespace-nowrap">
                              <span className="font-semibold block text-[var(--ink-primary)]">
                                {new Date(item.created_at).toLocaleDateString()}
                              </span>
                              <span className="text-[var(--ink-secondary)]">
                                {new Date(item.created_at).toLocaleTimeString([], {
                                  hour: "2-digit",
                                  minute: "2-digit",
                                })}
                              </span>
                            </TableCell>

                            <TableCell className="whitespace-nowrap">
                              <Badge
                                variant={
                                  item.target_type === "broadcast"
                                    ? "blue"
                                    : "subtle"
                                }
                                size="sm"
                              >
                                {item.target_type}
                              </Badge>
                              {item.target_id && (
                                <span className="block text-[10px] text-[var(--ink-tertiary)] font-mono mt-0.5 truncate max-w-[120px]">
                                  {item.target_id}
                                </span>
                              )}
                            </TableCell>

                            <TableCell className="max-w-xs md:max-w-md">
                              <p className="font-semibold text-xs text-[var(--ink-primary)]">
                                {item.title}
                              </p>
                              <p className="text-xs text-[var(--ink-secondary)] line-clamp-2 mt-0.5">
                                {item.body}
                              </p>
                              {item.action_url && (
                                <a
                                  href={item.action_url}
                                  className="inline-flex items-center gap-1 text-[11px] underline text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] mt-1"
                                >
                                  <span>{item.action_url}</span>
                                  <ExternalLink className="h-3 w-3" />
                                </a>
                              )}
                            </TableCell>

                            <TableCell className="text-xs font-mono text-[var(--ink-secondary)]">
                              {item.created_by_email || "System"}
                            </TableCell>

                            <TableCell className="text-right whitespace-nowrap">
                              <div className="font-semibold text-xs text-[var(--ink-primary)]">
                                {item.read_count} / {item.recipients_count}{" "}
                                <span className="text-[var(--ink-secondary)] font-normal">
                                  ({readPercentage}%)
                                </span>
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>

                  {/* Pagination */}
                  <div className="flex items-center justify-between pt-2">
                    <span className="text-xs text-[var(--ink-secondary)]">
                      Total: {historyData.total_count} records
                    </span>

                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={page <= 1}
                        onClick={() => setPage((p) => Math.max(1, p - 1))}
                        className="text-xs h-7"
                      >
                        Previous
                      </Button>
                      <span className="text-xs font-semibold">Page {page}</span>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={
                          page * limit >= (historyData.total_count || 0)
                        }
                        onClick={() => setPage((p) => p + 1)}
                        className="text-xs h-7"
                      >
                        Next
                      </Button>
                    </div>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
