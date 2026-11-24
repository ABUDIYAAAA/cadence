"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Calendar,
  ChevronRight,
  Plus,
  Shield,
  Trash2,
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Alert } from "@/components/ui/alert";
import { LogoUploadField } from "@/components/events/logo-upload-field";

interface DisplayEventCard {
  id: string;
  slug: string;
  name: string;
  description?: string | null;
  logo_url?: string | null;
  member_count: number;
  role: EventRole;
}

function EventsListSkeleton() {
  return (
    <div className="rounded-[10px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-card)] overflow-hidden divide-y divide-[var(--card-line)]">
      {Array.from({ length: 3 }).map((_, idx) => (
        <div key={idx} className="flex items-center justify-between gap-4 px-4 py-3.5">
          <div className="flex items-center gap-3.5 min-w-0 flex-1">
            <Skeleton className="h-10 w-10 shrink-0 rounded-[8px]" />
            <div className="space-y-2 flex-1 max-w-[220px]">
              <Skeleton className="h-4 w-3/4 rounded-[6px]" />
              <Skeleton className="h-3 w-1/2 rounded-[6px]" />
            </div>
          </div>
          <Skeleton className="h-4 w-4 shrink-0 rounded-[4px]" />
        </div>
      ))}
    </div>
  );
}

export default function EventsSelectionPage() {
  const router = useRouter();
  const toast = useToast();
  const { user, isAuthenticated, isLoading, isDev } = useAuth();
  const {
    eventSlug: activeEventSlug,
    events: userEvents,
    selectEvent,
    clearEvent,
  } = useTenant();

  const { data: allEvents = [], isLoading: eventsLoading } = useEvents({
    enabled: isAuthenticated,
  });

  const createEventMutation = useCreateEvent();
  const deleteEventMutation = useDeleteEvent();

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [description, setDescription] = useState("");
  const [logoUrl, setLogoUrl] = useState("");
  const [ownerEmail, setOwnerEmail] = useState("");
  const [ownerName, setOwnerName] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);

  const [deletingEvent, setDeletingEvent] = useState<DisplayEventCard | null>(
    null
  );

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  const cards = useMemo<DisplayEventCard[]>(() => {
    const bySlug = new Map(allEvents.map((e) => [e.slug, e]));
    const roleBySlug = new Map(userEvents.map((e) => [e.event_slug, e]));

    if (isDev && allEvents.length > 0) {
      return allEvents.map((ev) => ({
        id: ev.id,
        slug: ev.slug,
        name: ev.name,
        description: ev.description,
        logo_url: ev.logo_url ?? roleBySlug.get(ev.slug)?.logo_url ?? null,
        member_count: ev.member_count ?? roleBySlug.get(ev.slug)?.member_count ?? 0,
        role: roleBySlug.get(ev.slug)?.role ?? "owner",
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
      };
    });
  }, [allEvents, userEvents, isDev]);

  const handleSelectEvent = (item: DisplayEventCard) => {
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
  };

  const handleCreateEvent = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateError(null);

    const trimmedName = name.trim();
    const trimmedSlug = (slug.trim() || slugify(trimmedName)).toLowerCase();

    if (trimmedName.length < 2) {
      setCreateError("Event name must be at least 2 characters.");
      return;
    }
    if (trimmedSlug.length < 2) {
      setCreateError("Event slug must be at least 2 characters.");
      return;
    }

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

  if (isLoading || !isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-xl px-4 sm:px-6 py-10 sm:py-14 font-sans space-y-6">
        <div className="space-y-2">
          <Skeleton className="h-7 w-28 rounded-[6px]" />
          <Skeleton className="h-4 w-72 rounded-[6px]" />
        </div>
        <EventsListSkeleton />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-xl px-4 sm:px-6 py-10 sm:py-14 font-sans space-y-6">
      {/* Header */}
      <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2.5">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Events
            </h1>
            <Badge variant="subtle" size="sm">
              {cards.length} {cards.length === 1 ? "event" : "events"}
            </Badge>
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Select an event to open its workspace.
          </p>
        </div>

        {isDev && (
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-9 px-3 text-xs gap-1.5"
              onClick={() => router.push("/dashboard/users")}
            >
              <Shield className="h-3.5 w-3.5 text-[var(--color-amber)]" />
              Platform users
            </Button>
            <Button
              variant="primary"
              size="sm"
              className="h-9 px-3.5 text-xs gap-1.5 font-semibold"
              onClick={() => {
                resetCreateForm();
                setCreateOpen(true);
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              Create event
            </Button>
          </div>
        )}
      </header>

      {/* Centered Squared Event List */}
      {eventsLoading && cards.length === 0 ? (
        <EventsListSkeleton />
      ) : cards.length === 0 ? (
        <EmptyState
          title="No events linked to your email yet"
          description={`You are signed in as ${user.email}. Ask your event organizer to add this email address to their event.`}
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
      ) : (
        <div className="rounded-[10px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-card)] overflow-hidden divide-y divide-[var(--card-line)]">
          {cards.map((item) => {
            const isSelected = activeEventSlug === item.slug;

            return (
              <div
                key={item.slug}
                onClick={() => handleSelectEvent(item)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleSelectEvent(item);
                  }
                }}
                className={`group flex items-center justify-between gap-4 px-4 py-3.5 transition-colors duration-150 cursor-pointer hover:bg-[var(--card-solid-subtle)] ${
                  isSelected ? "bg-[var(--card-solid-subtle)]" : ""
                }`}
              >
                <div className="flex items-center gap-3.5 min-w-0 flex-1">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]">
                    {item.logo_url ? (
                      <img
                        src={item.logo_url}
                        alt={`${item.name} logo`}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <Calendar className="h-4 w-4 text-[var(--ink-secondary)]" />
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <h2 className="text-sm font-semibold tracking-tight text-[var(--ink-primary)] truncate">
                        {item.name}
                      </h2>
                      {isSelected && (
                        <Badge variant="emerald" dot size="sm">
                          Active
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-[var(--ink-secondary)] mt-0.5">
                      {item.member_count}{" "}
                      {item.member_count === 1 ? "member" : "members"}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  <Badge variant="subtle" size="sm" className="capitalize">
                    {item.role}
                  </Badge>

                  {isDev && (
                    <button
                      type="button"
                      aria-label={`Delete ${item.name}`}
                      title="Delete event"
                      className="flex h-7 w-7 items-center justify-center rounded-[6px] text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeletingEvent(item);
                      }}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  )}

                  <ChevronRight className="h-4 w-4 text-[var(--ink-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5 group-hover:text-[var(--ink-primary)]" />
                </div>
              </div>
            );
          })}
        </div>
      )}

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
              if (!slugTouched) {
                setSlug(slugify(val));
              }
            }}
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
            }}
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
            Leave owner email blank to assign yourself as the initial owner. An owner can only own one event at a time.
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
