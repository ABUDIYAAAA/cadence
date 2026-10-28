"use client";

import React, { useState, useMemo } from "react";
import {
  Send,
  Bell,
  History,
  ExternalLink,
  RefreshCw,
  Smartphone,
  Inbox,
  Users,
  Layers,
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
  const teams = teamsData?.data || [];

  // Fetch Users for dropdown (with optional search filter)
  const { data: usersData, isLoading: usersLoading } = useUsers({
    limit: 100,
    search: userSearchQuery.trim() || undefined,
  });
  const users = usersData?.data || [];

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
      <Alert variant="error" title="ACCESS RESTRICTED">
        Only administrators have authorization to dispatch broadcast notifications.
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
      setFormError(`Please select a ${targetType} from the dropdown.`);
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
        `Dispatched to ${resp.recipients_count} recipient(s). ${resp.enqueued_pushes} Web Push frame(s) enqueued.`,
        "NOTIFICATION PUBLISHED"
      );

      // Reset form fields
      setTitle("");
      setBody("");
      setActionUrl("");
      setIconUrl("");
      setTargetId("");
      refetchHistory();
    } catch (err: any) {
      const errorMsg =
        err?.error || err?.message || "Failed to publish notification.";
      setFormError(errorMsg);
      toast.error(errorMsg, "DISPATCH FAILED");
    }
  };

  return (
    <div className="translucent-container space-y-6 font-sans">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-[var(--card-line)]">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Badge variant="blue" className="text-[10px] tracking-wider uppercase font-semibold">
              ADMIN MESSAGING
            </Badge>
            <span className="text-xs text-[var(--ink-tertiary)]">RABBITMQ WEB PUSH</span>
          </div>
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
            Notification Publication
          </h1>
          <p className="text-xs text-[var(--ink-secondary)] mt-1">
            Dispatch announcements, emergency alerts, and room calls with live push delivery.
          </p>
        </div>

        <Badge variant="solid" size="md">
          BROADCASTER READY
        </Badge>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="compose">
        <TabsList>
          <TabsTrigger value="compose">
            <span className="flex items-center gap-1.5">
              <Send className="h-3.5 w-3.5" />
              <span>Compose & Dispatch</span>
            </span>
          </TabsTrigger>
          <TabsTrigger value="history">
            <span className="flex items-center gap-1.5">
              <History className="h-3.5 w-3.5" />
              <span>Dispatch History ({historyData?.total_count ?? 0})</span>
            </span>
          </TabsTrigger>
        </TabsList>

        {/* TAB 1: COMPOSE & DISPATCH */}
        <TabsContent value="compose">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
            {/* Form Column */}
            <div className="lg:col-span-7">
              <Card>
                <CardHeader>
                  <CardTitle>Publication Composer</CardTitle>
                  <CardDescription>
                    Configure notification target criteria and payload.
                  </CardDescription>
                </CardHeader>

                <form onSubmit={handleSubmit}>
                  <CardContent className="space-y-4">
                    {formError && (
                      <Alert variant="error" title="VALIDATION ERROR">
                        {formError}
                      </Alert>
                    )}

                    {/* Target Audience Segmented Buttons */}
                    <div className="space-y-1.5">
                      <label className="block text-xs font-semibold text-[var(--ink-secondary)]">
                        Target Audience
                      </label>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-1.5 p-1 rounded-[14px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)]">
                        {[
                          { id: "broadcast", label: "All Users" },
                          { id: "role", label: "By Role" },
                          { id: "team", label: "By Team" },
                          { id: "user", label: "Single User" },
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
                            className={`px-2.5 py-1.5 text-xs font-semibold rounded-[10px] transition-all cursor-pointer text-center ${
                              targetType === item.id
                                ? "bg-[var(--color-dark)] text-[var(--ink-inverse)] shadow-sm"
                                : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
                            }`}
                          >
                            {item.label}
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* TARGET TYPE: ROLE */}
                    {targetType === "role" && (
                      <div className="space-y-1 p-3 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px]">
                        <Select
                          label="Select Target Role"
                          value={roleSelection}
                          onChange={(e) => setRoleSelection(e.target.value)}
                        >
                          <option value="participant">Participant (Hackers / Attendees)</option>
                          <option value="judge">Judge (Panel Evaluators)</option>
                          <option value="volunteer">Volunteer (Staff & Coordinators)</option>
                          <option value="admin">Administrator</option>
                        </Select>
                        <p className="text-[11px] text-[var(--ink-secondary)] pt-1">
                          Notification will be delivered to every active user with the <strong>{roleSelection}</strong> role.
                        </p>
                      </div>
                    )}

                    {/* TARGET TYPE: TEAM DROPDOWN */}
                    {targetType === "team" && (
                      <div className="space-y-2 p-3 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px]">
                        <div className="flex items-center justify-between">
                          <label className="block text-xs uppercase tracking-wider font-bold text-[var(--ink-primary)] flex items-center gap-1.5">
                            <Layers className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                            <span>Select Target Team</span>
                          </label>
                          <button
                            type="button"
                            onClick={() => setIsManualTeamId(!isManualTeamId)}
                            className="text-[11px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] underline cursor-pointer"
                          >
                            {isManualTeamId ? "← Use Team Dropdown" : "Enter Manual UUID"}
                          </button>
                        </div>

                        {isManualTeamId ? (
                          <Input
                            placeholder="Paste Team UUID..."
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            helperText="Direct raw UUID targeting"
                          />
                        ) : (
                          <Select
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            disabled={teamsLoading}
                          >
                            <option value="">
                              {teamsLoading ? "Loading teams..." : "— Choose a Team from list —"}
                            </option>
                            {teams.map((t) => (
                              <option key={t.id} value={t.id}>
                                {t.name} ({t.member_count} member{t.member_count !== 1 ? "s" : ""})
                                {t.leader_name ? ` · Leader: ${t.leader_name}` : ""}
                              </option>
                            ))}
                          </Select>
                        )}

                        {selectedTeam && (
                          <div className="p-2.5 bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] text-xs flex items-center justify-between">
                            <div>
                              <span className="font-bold text-[var(--ink-primary)]">{selectedTeam.name}</span>
                              <span className="text-[var(--ink-secondary)] text-[11px] block">
                                Leader: {selectedTeam.leader_name || "Unassigned"} ({selectedTeam.leader_email || "no email"})
                              </span>
                            </div>
                            <Badge variant="solid" size="sm">
                              {selectedTeam.member_count} members
                            </Badge>
                          </div>
                        )}
                      </div>
                    )}

                    {/* TARGET TYPE: SINGLE USER DROPDOWN + SEARCH */}
                    {targetType === "user" && (
                      <div className="space-y-2 p-3 bg-[var(--card-solid-subtle)] border border-[var(--card-line)] rounded-[14px]">
                        <div className="flex items-center justify-between">
                          <label className="block text-xs uppercase tracking-wider font-bold text-[var(--ink-primary)] flex items-center gap-1.5">
                            <Users className="h-3.5 w-3.5 text-[var(--color-blue)]" />
                            <span>Select Recipient User</span>
                          </label>
                          <button
                            type="button"
                            onClick={() => setIsManualUserId(!isManualUserId)}
                            className="text-[11px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] underline cursor-pointer"
                          >
                            {isManualUserId ? "← Use User Dropdown" : "Enter Manual UUID"}
                          </button>
                        </div>

                        {isManualUserId ? (
                          <Input
                            placeholder="Paste User UUID..."
                            value={targetId}
                            onChange={(e) => setTargetId(e.target.value)}
                            helperText="Direct raw UUID targeting"
                          />
                        ) : (
                          <div className="space-y-2">
                            {/* Search filter input */}
                            <div className="relative">
                              <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-[var(--ink-tertiary)]" />
                              <input
                                type="text"
                                placeholder="Type to filter users by name or email..."
                                value={userSearchQuery}
                                onChange={(e) => setUserSearchQuery(e.target.value)}
                                className="w-full h-8 pl-8 pr-3 text-xs rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid)] text-[var(--ink-primary)] focus:outline-none focus:border-[var(--color-blue)]"
                              />
                            </div>

                            <Select
                              value={targetId}
                              onChange={(e) => setTargetId(e.target.value)}
                              disabled={usersLoading}
                            >
                              <option value="">
                                {usersLoading
                                  ? "Loading users..."
                                  : users.length === 0
                                  ? "No users match search"
                                  : "— Select a User from list —"}
                              </option>
                              {users.map((u) => (
                                <option key={u.id} value={u.id}>
                                  {u.name} — {u.email} [{u.role.toUpperCase()}]
                                </option>
                              ))}
                            </Select>
                          </div>
                        )}

                        {selectedUser && (
                          <div className="p-2.5 bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[12px] text-xs flex items-center justify-between">
                            <div>
                              <span className="font-bold text-[var(--ink-primary)]">{selectedUser.name}</span>
                              <span className="text-[var(--ink-secondary)] text-[11px] block font-mono">
                                {selectedUser.email}
                              </span>
                            </div>
                            <Badge variant="solid" size="sm">
                              {selectedUser.role}
                            </Badge>
                          </div>
                        )}
                      </div>
                    )}

                    {/* Notification Title */}
                    <Input
                      label="Notification Title"
                      required
                      placeholder="e.g. Round 1 Judging Starting in 10 Minutes"
                      value={title}
                      maxLength={255}
                      onChange={(e) => setTitle(e.target.value)}
                      helperText={`${title.length}/255 characters`}
                    />

                    {/* Notification Body */}
                    <Textarea
                      label="Message Content"
                      required
                      rows={4}
                      placeholder="Enter detailed announcement, room assignments, instructions, or urgent updates..."
                      value={body}
                      onChange={(e) => setBody(e.target.value)}
                    />

                    {/* Deep Link URL & Icon */}
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <Input
                        label="Action URL (Optional)"
                        placeholder="/dashboard/judging"
                        value={actionUrl}
                        onChange={(e) => setActionUrl(e.target.value)}
                        helperText="Deep link opened on click"
                      />

                      <Input
                        label="Icon URL (Optional)"
                        placeholder="/favicon.ico"
                        value={iconUrl}
                        onChange={(e) => setIconUrl(e.target.value)}
                        helperText="Custom push icon"
                      />
                    </div>
                  </CardContent>

                  <CardFooter className="flex flex-col sm:flex-row justify-between items-center gap-3 pt-3 border-t border-[var(--card-line)]">
                    <span className="text-xs text-[var(--ink-secondary)]">
                      Dispatched over RabbitMQ Web Push workers.
                    </span>

                    <Button
                      type="submit"
                      variant="primary"
                      size="md"
                      isLoading={broadcastMutation.isPending}
                      className="w-full sm:w-auto px-6 font-bold"
                    >
                      <Send className="h-4 w-4 mr-2" />
                      <span>Dispatch Broadcast</span>
                    </Button>
                  </CardFooter>
                </form>
              </Card>
            </div>

            {/* Live Preview Column */}
            <div className="lg:col-span-5 space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Smartphone className="h-4 w-4 text-[var(--color-blue)]" />
                    <span>Live Push Frame Preview</span>
                  </CardTitle>
                  <CardDescription>
                    Client preview rendered as browser push frame.
                  </CardDescription>
                </CardHeader>

                <CardContent className="space-y-4">
                  {/* Push Frame Mockup */}
                  <div className="rounded-[18px] border border-[var(--card-border)] bg-[var(--card-solid)] p-4.5 shadow-[0_8px_24px_-4px_rgba(16,20,26,0.1)] text-[var(--ink-primary)] space-y-2.5">
                    <div className="flex items-center justify-between border-b border-[var(--card-line)] pb-2">
                      <div className="flex items-center gap-2">
                        <span className="h-5 w-5 rounded-md bg-[var(--color-dark)] text-[var(--ink-inverse)] text-[10px] flex items-center justify-center font-bold">
                          C
                        </span>
                        <span className="text-[11px] font-bold tracking-wider text-[var(--ink-primary)]">
                          CADENCE · NOW
                        </span>
                      </div>
                      <Badge variant="blue" size="sm">
                        PUSH
                      </Badge>
                    </div>

                    <div>
                      <h4 className="text-xs font-bold text-[var(--ink-primary)] break-words">
                        {title.trim() || "Notification Title Appears Here"}
                      </h4>
                      <p className="text-xs text-[var(--ink-secondary)] mt-1 leading-relaxed break-words">
                        {body.trim() ||
                          "Message content body will be displayed here in full detail when delivered to user devices."}
                      </p>
                    </div>

                    {actionUrl && (
                      <div className="pt-2 border-t border-[var(--card-line)] flex items-center gap-1 text-[11px] font-bold text-[var(--color-blue)] hover:underline">
                        <span>Open: {actionUrl}</span>
                        <ExternalLink className="h-3 w-3" />
                      </div>
                    )}
                  </div>

                  {/* In-App Feed Mockup */}
                  <div className="rounded-[16px] border border-[var(--card-line)] p-3.5 bg-[var(--card-solid-subtle)] space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider text-[var(--ink-tertiary)]">
                        <Inbox className="h-3.5 w-3.5" />
                        <span>In-App Feed Preview</span>
                      </span>
                      <span className="text-[10px] bg-[var(--color-dark)] text-[var(--ink-inverse)] rounded-full px-2 py-0.5 font-bold">
                        UNREAD
                      </span>
                    </div>

                    <div>
                      <p className="text-xs font-bold text-[var(--ink-primary)]">
                        {title.trim() || "Notification Title"}
                      </p>
                      <p className="text-[11px] text-[var(--ink-secondary)] mt-0.5">
                        {body.trim() || "Message details displayed in inbox feed."}
                      </p>
                    </div>
                  </div>

                  {/* Scope Summary Box */}
                  <div className="rounded-[16px] border border-[var(--card-line)] p-3.5 text-xs space-y-2 bg-[var(--card-solid-subtle)]">
                    <div className="flex justify-between items-center py-0.5">
                      <span className="text-[var(--ink-secondary)]">Target Audience:</span>
                      <Badge variant="outline" size="sm">
                        {targetType === "broadcast"
                          ? "All Registered Users"
                          : targetType === "role"
                          ? `Role: ${roleSelection}`
                          : targetType === "team"
                          ? `Team: ${selectedTeam?.name || targetId || "Not Selected"}`
                          : `User: ${selectedUser?.name || targetId || "Not Selected"}`}
                      </Badge>
                    </div>
                    <div className="flex justify-between items-center py-0.5">
                      <span className="text-[var(--ink-secondary)]">Sender Identity:</span>
                      <span className="font-bold text-[var(--ink-primary)]">{user?.email}</span>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        </TabsContent>

        {/* TAB 2: DISPATCH HISTORY */}
        <TabsContent value="history">
          <Card>
            <CardHeader>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div>
                  <CardTitle>Historical Dispatches</CardTitle>
                  <CardDescription>
                    All broadcast and targeted push records with read tracking.
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
                  <span>Refresh History</span>
                </Button>
              </div>
            </CardHeader>

            <CardContent>
              {historyLoading ? (
                <div className="p-8 text-center text-xs text-[var(--ink-secondary)]">
                  Loading notification dispatch logs...
                </div>
              ) : !historyData?.items || historyData.items.length === 0 ? (
                <EmptyState
                  title="No Broadcasts Dispatched"
                  description="Use the composer tab to publish your first announcement or round update."
                />
              ) : (
                <div className="space-y-4">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Timestamp</TableHead>
                        <TableHead>Target Scope</TableHead>
                        <TableHead>Message & Payload</TableHead>
                        <TableHead>Dispatched By</TableHead>
                        <TableHead className="text-right">Reach & Reads</TableHead>
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
                              <span className="font-bold block text-[var(--ink-primary)]">
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
                                    ? "solid"
                                    : "outline"
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
                              <p className="font-bold text-xs text-[var(--ink-primary)]">
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
                              <div className="font-bold text-xs text-[var(--ink-primary)]">
                                {item.read_count} / {item.recipients_count}{" "}
                                <span className="text-[var(--ink-secondary)] font-normal">
                                  ({readPercentage}%)
                                </span>
                              </div>
                              <span className="text-[10px] text-[var(--ink-tertiary)] uppercase font-semibold">
                                Delivered
                              </span>
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
                      <span className="text-xs font-bold">Page {page}</span>
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
