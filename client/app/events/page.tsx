"use client";

import React, { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  Calendar,
  Crown,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useCreateEvent,
  useDeleteEvent,
  useEvents,
} from "@/hooks/use-events";
import { getDashboardRoute } from "@/lib/auth-routes";
import { slugify } from "@/lib/csv";
import { apiErrorMessage } from "@/lib/utils";
import { EventRole } from "@/types/auth";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { LogoUploadField } from "@/components/events/logo-upload-field";

interface DisplayEventRow {
  id: string;
  slug: string;
  name: string;
  description?: string | null;
  logo_url?: string | null;
  member_count: number;
  role: EventRole;
  updated_at?: string | null;
}

function EventsTableSkeleton() {
  return (
    <div className="w-full overflow-x-auto font-sans">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Event</TableHead>
            <TableHead className="hidden sm:table-cell">Slug</TableHead>
            <TableHead>Members</TableHead>
            <TableHead>Role</TableHead>
            <TableHead className="hidden lg:table-cell">Updated</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {Array.from({ length: 4 }).map((_, i) => (
            <TableRow key={i}>
              <TableCell>
                <div className="flex items-center gap-3">
                  <Skeleton className="h-9 w-9 shrink-0 rounded-[8px]" />
                  <div className="space-y-1.5">
                    <Skeleton className="h-3.5 w-36" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                </div>
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                <Skeleton className="h-3.5 w-28" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-3.5 w-20" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-16 rounded-full" />
              </TableCell>
              <TableCell className="hidden lg:table-cell">
                <Skeleton className="h-3.5 w-24" />
              </TableCell>
              <TableCell className="text-right">
                <div className="inline-flex items-center gap-1.5">
                  <Skeleton className="h-8 w-16 rounded-[8px]" />
                  <Skeleton className="h-8 w-8 rounded-[8px]" />
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function renderRoleBadge(role: EventRole) {
  switch (role) {
    case "owner":
      return (
        <Badge variant="blue" size="sm" className="capitalize gap-1">
          <Crown className="h-2.5 w-2.5" />
          {role}
        </Badge>
      );
    case "admin":
      return (
        <Badge variant="emerald" size="sm" className="capitalize">
          {role}
        </Badge>
      );
    default:
      return (
        <Badge variant="subtle" size="sm" className="capitalize">
          {role}
        </Badge>
      );
  }
}

export default function EventsSelectionPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAuthenticated, isDev } = useAuth();
  const {
    eventSlug: activeEventSlug,
    events: userEvents,
    selectEvent,
    clearEvent,
  } = useTenant();

  const {
    data: allEvents = [],
    isLoading: eventsLoading,
    isFetching,
    isError,
    error,
    refetch,
  } = useEvents({
    enabled: isAuthenticated,
  });

  const createEventMutation = useCreateEvent();
  const deleteEventMutation = useDeleteEvent();

  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [description, setDescription] = useState("");
  const [logoUrl, setLogoUrl] = useState("");
  const [ownerEmail, setOwnerEmail] = useState("");
  const [ownerName, setOwnerName] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{ name?: string; slug?: string }>({});

  const [deletingEvent, setDeletingEvent] = useState<DisplayEventRow | null>(
    null
  );

  const rows = useMemo<DisplayEventRow[]>(() => {
    const bySlug = new Map(allEvents.map((e) => [e.slug, e]));
    const roleBySlug = new Map(userEvents.map((e) => [e.event_slug, e]));

    if (isDev && allEvents.length > 0) {
      return allEvents.map((ev) => ({
        id: ev.id,
        slug: ev.slug,
        name: ev.name,
        description: ev.description,
        logo_url: ev.logo_url ?? roleBySlug.get(ev.slug)?.logo_url ?? null,
        member_count:
          ev.member_count ?? roleBySlug.get(ev.slug)?.member_count ?? 0,
        role: roleBySlug.get(ev.slug)?.role ?? "owner",
        updated_at: ev.updated_at ?? ev.created_at ?? null,
      }));
    }

    return userEvents.map((ue) => {
      const full = bySlug.get(ue.event_slug);
      return {
        id: ue.event_id || full?.id || ue.event_slug,
        slug: ue.event_slug,
        name: full?.name || ue.event_name || ue.event_slug,
        description: full?.description ?? null,
        logo_url: full?.logo_url ?? ue.logo_url ?? null,
        member_count: full?.member_count ?? ue.member_count ?? 0,
        role: ue.role,
        updated_at: full?.updated_at ?? full?.created_at ?? null,
      };
    });
  }, [allEvents, userEvents, isDev]);

  const filteredRows = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (item) =>
        item.name.toLowerCase().includes(q) ||
        item.slug.toLowerCase().includes(q) ||
        (item.description && item.description.toLowerCase().includes(q))
    );
  }, [rows, searchQuery]);

  const activeRow = useMemo(
    () => rows.find((r) => r.slug === activeEventSlug) ?? null,
    [rows, activeEventSlug]
  );

  const handleSelectEvent = (item: DisplayEventRow) => {
    selectEvent(item.slug);
    const effectiveRole = isDev ? "owner" : item.role;
    router.push(getDashboardRoute(effectiveRole));
  };

  const resetCreateForm = () => {
    setName("");
    setSlug("");
    setSlugTouched(false);
    setDescription("");
    setLogoUrl("");
    setOwnerEmail("");
    setOwnerName("");
    setCreateError(null);
    setFieldErrors({});
  };

  const handleCreateEvent = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateError(null);

    const trimmedName = name.trim();
    const trimmedSlug = (slug.trim() || slugify(trimmedName)).toLowerCase();

    const nextErrors: { name?: string; slug?: string } = {};
    if (trimmedName.length < 2) {
      nextErrors.name = trimmedName ? "At least 2 characters" : "Required";
    }
    if (trimmedSlug.length < 2) {
      nextErrors.slug = trimmedSlug ? "At least 2 characters" : "Required";
    }
    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors);
      return;
    }
    setFieldErrors({});

    try {
      const created = await createEventMutation.mutateAsync({
        name: trimmedName,
        slug: trimmedSlug,
        description: description.trim() || undefined,
        logo_url: logoUrl.trim() || undefined,
        owner_email: ownerEmail.trim() || undefined,
        owner_name: ownerName.trim() || undefined,
      });
      toast.success(`Event "${created.name}" created.`, "Event created");
      setCreateOpen(false);
      resetCreateForm();
    } catch (err: unknown) {
      setCreateError(apiErrorMessage(err, "Failed to create event"));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deletingEvent) return;
    try {
      await deleteEventMutation.mutateAsync(deletingEvent.slug);
      if (activeEventSlug === deletingEvent.slug) {
        clearEvent();
      }
      toast.info(`Event "${deletingEvent.name}" deleted.`, "Event deleted");
      setDeletingEvent(null);
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to delete event"), "Error");
    }
  };

  return (
    <div className="space-y-6 font-sans">
      {/* Header */}
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Events
            </h1>
            <Badge variant="subtle" size="sm">
              {rows.length} {rows.length === 1 ? "event" : "events"}
            </Badge>
            {activeRow && (
              <Badge variant="emerald" dot size="sm">
                Active: {activeRow.name}
              </Badge>
            )}
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            {isDev
              ? "Manage platform events and open their workspaces."
              : "Select an event to open its workspace."}
          </p>
        </div>

        {/* Action Toolbar */}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh events"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {isDev && (
            <Button
              variant="primary"
              size="sm"
              className="h-9 px-4 text-xs gap-1.5 font-semibold"
              onClick={() => {
                resetCreateForm();
                setCreateOpen(true);
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              <span>Create event</span>
            </Button>
          )}
        </div>
      </header>

      {/* Error State */}
      {isError && (
        <Alert variant="error" title="Could not load events">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <span>
              {apiErrorMessage(error, "Failed to fetch events from the server.")}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              className="h-8 px-3 text-xs shrink-0"
            >
              Try again
            </Button>
          </div>
        </Alert>
      )}

      {/* Primary Surface: Search + Events Table */}
      <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4">
        {rows.length > 0 && (
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="relative flex-1 max-w-md">
              <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--ink-tertiary)]" />
              <Input
                placeholder="Search events by name, slug, or description..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 pr-8 text-xs h-9"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery("")}
                  aria-label="Clear search"
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[var(--ink-tertiary)] hover:text-[var(--ink-primary)] cursor-pointer"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>

            {searchQuery.trim() && (
              <p className="text-xs text-[var(--ink-secondary)] tabular-nums">
                Showing {filteredRows.length} of {rows.length}{" "}
                {rows.length === 1 ? "event" : "events"}
              </p>
            )}
          </div>
        )}

        {eventsLoading && rows.length === 0 ? (
          <EventsTableSkeleton />
        ) : rows.length === 0 ? (
          <EmptyState
            title="No events linked to your account yet"
            description={
              user?.email
                ? `You are signed in as ${user.email}. ${
                    isDev
                      ? "Create your first event to provision a workspace."
                      : "Ask your event organizer to add this email address to their event."
                  }`
                : "No events are available yet."
            }
            action={
              isDev ? (
                <Button
                  variant="primary"
                  size="sm"
                  className="gap-1.5"
                  onClick={() => {
                    resetCreateForm();
                    setCreateOpen(true);
                  }}
                >
                  <Plus className="h-3.5 w-3.5" />
                  Create first event
                </Button>
              ) : undefined
            }
          />
        ) : filteredRows.length === 0 ? (
          <EmptyState
            title="No matching events"
            description={`No events matched "${searchQuery}".`}
            action={
              <Button
                variant="outline"
                size="sm"
                onClick={() => setSearchQuery("")}
              >
                Clear search
              </Button>
            }
          />
        ) : (
          <div className="w-full overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Event</TableHead>
                  <TableHead className="hidden sm:table-cell">Slug</TableHead>
                  <TableHead>Members</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead className="hidden lg:table-cell">
                    Updated
                  </TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredRows.map((item) => {
                  const isSelected = activeEventSlug === item.slug;

                  return (
                    <TableRow
                      key={item.slug}
                      onClick={() => handleSelectEvent(item)}
                      className={`group cursor-pointer ${
                        isSelected ? "bg-[var(--card-solid-subtle)]" : ""
                      }`}
                    >
                      {/* Event Identity */}
                      <TableCell>
                        <div className="flex items-center gap-3 min-w-0">
                          <div className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
                            {item.logo_url ? (
                              // eslint-disable-next-line @next/next/no-img-element
                              <img
                                src={item.logo_url}
                                alt={`${item.name} logo`}
                                className="h-full w-full object-cover"
                              />
                            ) : (
                              <Calendar className="h-4 w-4 text-[var(--ink-secondary)]" />
                            )}
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-semibold text-[var(--ink-primary)] truncate group-hover:text-[var(--color-blue)] transition-colors">
                                {item.name}
                              </span>
                              {isSelected && (
                                <Badge variant="emerald" dot size="sm">
                                  Active
                                </Badge>
                              )}
                            </div>
                            {item.description ? (
                              <p className="text-xs text-[var(--ink-secondary)] truncate max-w-[340px] mt-0.5">
                                {item.description}
                              </p>
                            ) : (
                              <p className="text-xs text-[var(--ink-tertiary)] sm:hidden font-mono mt-0.5 truncate">
                                {item.slug}
                              </p>
                            )}
                          </div>
                        </div>
                      </TableCell>

                      {/* Slug */}
                      <TableCell className="hidden sm:table-cell font-mono text-xs text-[var(--ink-secondary)] whitespace-nowrap">
                        {item.slug}
                      </TableCell>

                      {/* Members */}
                      <TableCell className="text-xs tabular-nums whitespace-nowrap">
                        <span className="font-semibold text-[var(--ink-primary)]">
                          {item.member_count}
                        </span>{" "}
                        <span className="text-[var(--ink-secondary)]">
                          {item.member_count === 1 ? "member" : "members"}
                        </span>
                      </TableCell>

                      {/* Role */}
                      <TableCell className="whitespace-nowrap">
                        {renderRoleBadge(item.role)}
                      </TableCell>

                      {/* Updated */}
                      <TableCell className="hidden lg:table-cell text-xs tabular-nums text-[var(--ink-secondary)] whitespace-nowrap">
                        {item.updated_at
                          ? new Date(item.updated_at).toLocaleDateString([], {
                              month: "short",
                              day: "numeric",
                              year: "numeric",
                            })
                          : "—"}
                      </TableCell>

                      {/* Actions */}
                      <TableCell
                        className="text-right whitespace-nowrap"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div className="inline-flex items-center justify-end gap-1.5">
                          <Button
                            variant="outline"
                            size="sm"
                            type="button"
                            onClick={() => handleSelectEvent(item)}
                            className="h-8 px-2.5 text-xs gap-1 font-medium"
                          >
                            <span>Open</span>
                            <ArrowRight className="h-3 w-3 text-[var(--ink-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5 group-hover:text-[var(--ink-primary)]" />
                          </Button>

                          {isDev && (
                            <button
                              type="button"
                              aria-label={`Delete ${item.name}`}
                              title="Delete event"
                              className="flex h-8 w-8 items-center justify-center rounded-[8px] text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer"
                              onClick={() => setDeletingEvent(item)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      {/* Dev: Create Event Dialog */}
      <Dialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Create Event"
        description="Provision a new top-level event and assign its initial owner."
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => setCreateOpen(false)}
              disabled={createEventMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={handleCreateEvent}
              isLoading={createEventMutation.isPending}
            >
              Create Event
            </Button>
          </div>
        }
      >
        <form onSubmit={handleCreateEvent} className="space-y-3 font-sans text-xs">
          {createError && (
            <Alert variant="error" title="Could not create event">
              {createError}
            </Alert>
          )}
          <Input
            label="Event Name *"
            placeholder="e.g. DevClub Hackathon 2026"
            value={name}
            onChange={(e) => {
              const val = e.target.value;
              setName(val);
              if (fieldErrors.name) setFieldErrors((prev) => ({ ...prev, name: undefined }));
              if (!slugTouched) {
                setSlug(slugify(val));
                if (fieldErrors.slug) setFieldErrors((prev) => ({ ...prev, slug: undefined }));
              }
            }}
            error={fieldErrors.name}
            disabled={createEventMutation.isPending}
            required
          />
          <Input
            label="Event Slug *"
            placeholder="e.g. devclub-hackathon-2026"
            value={slug}
            onChange={(e) => {
              setSlugTouched(true);
              setSlug(e.target.value.toLowerCase());
              if (fieldErrors.slug) setFieldErrors((prev) => ({ ...prev, slug: undefined }));
            }}
            error={fieldErrors.slug}
            disabled={createEventMutation.isPending}
            required
          />
          <Textarea
            label="Description"
            placeholder="Short summary of the event..."
            rows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={createEventMutation.isPending}
          />
          <LogoUploadField
            value={logoUrl}
            onChange={setLogoUrl}
            disabled={createEventMutation.isPending}
          />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-[var(--card-line)]">
            <Input
              label="Initial Owner Email (optional)"
              type="email"
              placeholder="owner@event.com"
              value={ownerEmail}
              onChange={(e) => setOwnerEmail(e.target.value)}
              disabled={createEventMutation.isPending}
            />
            <Input
              label="Initial Owner Name (optional)"
              placeholder="Owner Name"
              value={ownerName}
              onChange={(e) => setOwnerName(e.target.value)}
              disabled={createEventMutation.isPending}
            />
          </div>
          <p className="text-[11px] text-[var(--ink-tertiary)]">
            Leave owner email blank to assign yourself as the initial owner. An
            owner can only own one event at a time.
          </p>
        </form>
      </Dialog>

      {/* Dev: Delete Event Dialog */}
      <Dialog
        open={Boolean(deletingEvent)}
        onClose={() => setDeletingEvent(null)}
        title="Delete Event?"
        description={
          deletingEvent
            ? `Permanently delete "${deletingEvent.name}" and all of its competitions, teams, tickets, and judging rounds.`
            : ""
        }
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDeletingEvent(null)}
              disabled={deleteEventMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleConfirmDelete}
              isLoading={deleteEventMutation.isPending}
            >
              Delete Event
            </Button>
          </div>
        }
      >
        <p className="text-xs text-[var(--ink-secondary)]">
          This action is permanent and cannot be undone.
        </p>
      </Dialog>
    </div>
  );
}
