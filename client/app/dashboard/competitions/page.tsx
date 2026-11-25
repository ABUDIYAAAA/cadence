"use client";

import React, { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ChevronRight,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Trophy,
  X,
} from "lucide-react";
import { useAuth } from "@/providers/auth-provider";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  useCompetitions,
  useCreateCompetition,
  useDeleteCompetition,
  useUpdateCompetition,
} from "@/hooks/use-competitions";
import {
  getCompetitionSectionHref,
  resolveNextActiveCompetitionSlug,
  validateCompetitionInput,
} from "@/lib/competition-utils";
import { slugify } from "@/lib/csv";
import { apiErrorMessage } from "@/lib/utils";
import type { Competition } from "@/types/competitions";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";

function CompetitionsListSkeleton() {
  return (
    <div className="rounded-[12px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-card)] overflow-hidden divide-y divide-[var(--card-line)]">
      {Array.from({ length: 3 }).map((_, idx) => (
        <div
          key={idx}
          className="flex items-center justify-between gap-4 px-4 sm:px-5 py-4"
        >
          <div className="flex items-center gap-3.5 min-w-0 flex-1">
            <Skeleton className="h-10 w-10 shrink-0 rounded-[8px]" />
            <div className="space-y-2 flex-1 max-w-xs">
              <Skeleton className="h-4 w-3/4 rounded-[6px]" />
              <Skeleton className="h-3 w-1/2 rounded-[6px]" />
            </div>
          </div>
          <Skeleton className="h-7 w-20 shrink-0 rounded-[6px]" />
        </div>
      ))}
    </div>
  );
}

export default function CompetitionsPage() {
  const router = useRouter();
  const toast = useToast();
  const {
    user,
    isAuthenticated,
    isLoading: authLoading,
    isAdmin,
    isOwner,
    isDev,
    isVolunteer,
    isJudge,
    isParticipant,
  } = useAuth();
  const {
    eventSlug,
    activeEvent,
    competitionSlug: activeCompetitionSlug,
    selectCompetition,
  } = useTenant();

  const {
    data: competitions = [],
    isLoading: competitionsLoading,
    isFetching,
    isError: competitionsError,
    error: competitionsErrorObj,
    refetch,
  } = useCompetitions(eventSlug, {
    enabled: isAuthenticated && Boolean(eventSlug),
  });

  const createCompetition = useCreateCompetition(eventSlug);
  const updateCompetition = useUpdateCompetition(eventSlug);
  const deleteCompetition = useDeleteCompetition(eventSlug);

  const canManage = isOwner || isAdmin || isDev;
  const displayEventName = activeEvent?.event_name || eventSlug || "this event";

  const [searchQuery, setSearchQuery] = useState("");

  // Create / Edit Dialog state
  const [dialogMode, setDialogMode] = useState<"create" | "edit" | null>(null);
  const [editingCompetition, setEditingCompetition] =
    useState<Competition | null>(null);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [description, setDescription] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{ name?: string; slug?: string }>({});

  // Delete confirmation Dialog state
  const [deletingCompetition, setDeletingCompetition] =
    useState<Competition | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [authLoading, isAuthenticated, router]);

  const filteredCompetitions = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return competitions;
    return competitions.filter(
      (comp) =>
        comp.name.toLowerCase().includes(q) ||
        comp.slug.toLowerCase().includes(q) ||
        (comp.description && comp.description.toLowerCase().includes(q))
    );
  }, [competitions, searchQuery]);

  const openCreateDialog = () => {
    setEditingCompetition(null);
    setName("");
    setSlug("");
    setSlugTouched(false);
    setDescription("");
    setFormError(null);
    setFieldErrors({});
    setDialogMode("create");
  };

  const openEditDialog = (comp: Competition) => {
    setEditingCompetition(comp);
    setName(comp.name);
    setSlug(comp.slug);
    setSlugTouched(true);
    setDescription(comp.description || "");
    setFormError(null);
    setFieldErrors({});
    setDialogMode("edit");
  };

  const closeFormDialog = () => {
    if (createCompetition.isPending || updateCompetition.isPending) return;
    setDialogMode(null);
    setEditingCompetition(null);
    setFormError(null);
    setFieldErrors({});
  };

  const handleOpenCompetition = (comp: Competition) => {
    selectCompetition(comp.slug);
    const participantOnly =
      isParticipant && !isAdmin && !isVolunteer && !isJudge;
    const defaultSection = participantOnly ? "judging" : "teams";
    router.push(getCompetitionSectionHref(comp.slug, defaultSection));
  };

  const handleSaveCompetition = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    const trimmedName = name.trim();
    const trimmedSlug = (slug.trim() || slugify(trimmedName)).toLowerCase();
    const trimmedDescription = description.trim();

    const validationError = validateCompetitionInput({
      name: trimmedName,
      slug: trimmedSlug,
      description: trimmedDescription,
    });
    if (validationError) {
      const nextErrors: { name?: string; slug?: string } = {};
      if (!trimmedName || trimmedName.length < 2) {
        nextErrors.name = trimmedName ? "At least 2 characters" : "Required";
      }
      if (!trimmedSlug || validationError.toLowerCase().includes("slug")) {
        nextErrors.slug = trimmedSlug ? validationError : "Required";
      }
      if (Object.keys(nextErrors).length > 0) {
        setFieldErrors(nextErrors);
      } else {
        setFormError(validationError);
      }
      return;
    }
    setFieldErrors({});

    try {
      if (dialogMode === "create") {
        const created = await createCompetition.mutateAsync({
          name: trimmedName,
          slug: trimmedSlug,
          description: trimmedDescription || undefined,
        });
        selectCompetition(created.slug);
        toast.success(
          `Competition "${created.name}" created.`,
          "Competition created"
        );
      } else if (dialogMode === "edit" && editingCompetition) {
        const oldSlug = editingCompetition.slug;
        const updated = await updateCompetition.mutateAsync({
          competitionSlug: oldSlug,
          payload: {
            name: trimmedName,
            slug: trimmedSlug,
            description: trimmedDescription,
          },
        });
        const nextActive = resolveNextActiveCompetitionSlug({
          currentActiveSlug: activeCompetitionSlug,
          competitions,
          editedOldSlug: oldSlug,
          editedNewSlug: updated.slug,
        });
        if (nextActive !== activeCompetitionSlug) {
          selectCompetition(nextActive);
        }
        toast.success(
          `Competition "${updated.name}" updated.`,
          "Competition updated"
        );
      }
      setDialogMode(null);
      setEditingCompetition(null);
    } catch (err: unknown) {
      setFormError(apiErrorMessage(err, "Failed to save competition"));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deletingCompetition) return;
    setDeleteError(null);
    const targetSlug = deletingCompetition.slug;
    const targetName = deletingCompetition.name;

    try {
      await deleteCompetition.mutateAsync(targetSlug);
      const nextActive = resolveNextActiveCompetitionSlug({
        currentActiveSlug: activeCompetitionSlug,
        competitions,
        deletedSlug: targetSlug,
      });
      selectCompetition(nextActive);
      toast.info(
        `Competition "${targetName}" deleted.`,
        "Competition deleted"
      );
      setDeletingCompetition(null);
    } catch (err: unknown) {
      setDeleteError(apiErrorMessage(err, "Failed to delete competition"));
    }
  };

  if (authLoading || !isAuthenticated || !user) {
    return (
      <div className="space-y-6 font-sans">
        <div className="space-y-2">
          <Skeleton className="h-7 w-40 rounded-[6px]" />
          <Skeleton className="h-4 w-80 rounded-[6px]" />
        </div>
        <CompetitionsListSkeleton />
      </div>
    );
  }

  return (
    <div className="space-y-6 font-sans">
      {/* Page Header */}
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-[var(--ink-primary)]">
              Competitions
            </h1>
            <Badge variant="subtle" size="sm">
              {competitions.length}{" "}
              {competitions.length === 1 ? "competition" : "competitions"}
            </Badge>
          </div>
          <p className="text-sm text-[var(--ink-secondary)] mt-1">
            Open a competition in{" "}
            <strong className="font-medium text-[var(--ink-primary)]">
              {displayEventName}
            </strong>{" "}
            to manage its teams and judging rounds.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="h-9 px-3 text-xs gap-1.5"
            title="Refresh competitions"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          {canManage && (
            <Button
              variant="primary"
              size="sm"
              onClick={openCreateDialog}
              className="h-9 px-3.5 text-xs gap-1.5 font-semibold"
            >
              <Plus className="h-3.5 w-3.5" />
              <span>Create competition</span>
            </Button>
          )}
        </div>
      </header>

      {competitionsError && (
        <Alert variant="error" title="Could not load competitions">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <span>{apiErrorMessage(competitionsErrorObj, "Failed to fetch competitions from the server.")}</span>
            <Button variant="outline" size="sm" onClick={() => refetch()} className="h-8 px-3 text-xs shrink-0">
              Try again
            </Button>
          </div>
        </Alert>
      )}

      {/* Search Bar (shown when there are multiple competitions) */}
      {competitions.length > 1 && (
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--ink-tertiary)]" />
          <Input
            placeholder="Search competitions by name, slug, or description..."
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
      )}

      {/* Competition List */}
      {competitionsLoading && competitions.length === 0 ? (
        <CompetitionsListSkeleton />
      ) : competitions.length === 0 ? (
        <EmptyState
          title="No competitions in this event yet"
          description={
            canManage
              ? "Create a competition to enable team registration, submissions, and judging rounds."
              : "Organizers have not published a competition for this event yet."
          }
          action={
            canManage ? (
              <Button
                variant="primary"
                size="sm"
                className="gap-1.5"
                onClick={openCreateDialog}
              >
                <Plus className="h-3.5 w-3.5" />
                Create first competition
              </Button>
            ) : undefined
          }
        />
      ) : filteredCompetitions.length === 0 ? (
        <EmptyState
          title="No matching competitions"
          description={`No competitions matched "${searchQuery}".`}
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
        <div className="rounded-[12px] border border-[var(--card-border)] bg-[var(--card-solid)] shadow-[var(--shadow-card)] overflow-hidden divide-y divide-[var(--card-line)]">
          {filteredCompetitions.map((comp) => {
            return (
              <div
                key={comp.id}
                onClick={() => handleOpenCompetition(comp)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleOpenCompetition(comp);
                  }
                }}
                className="group flex items-center justify-between gap-4 px-4 sm:px-5 py-4 transition-colors duration-150 cursor-pointer hover:bg-[var(--card-solid-subtle)] focus-visible:outline-none focus-visible:bg-[var(--card-solid-subtle)]"
              >
                <div className="flex items-center gap-3.5 min-w-0 flex-1">
                  <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-[var(--ink-secondary)] transition-colors duration-150 group-hover:border-[var(--color-blue-border)] group-hover:bg-[var(--color-blue-dim)] group-hover:text-[var(--color-blue)]">
                    <Trophy className="h-4 w-4" />
                  </div>

                  <div className="min-w-0 flex-1 space-y-0.5">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-sm font-semibold tracking-tight text-[var(--ink-primary)] group-hover:text-[var(--color-blue)] transition-colors truncate">
                        {comp.name}
                      </h2>
                      <span className="font-mono text-[11px] text-[var(--ink-tertiary)] truncate">
                        /{comp.slug}
                      </span>
                    </div>

                    {comp.description ? (
                      <p className="text-xs text-[var(--ink-secondary)] line-clamp-1">
                        {comp.description}
                      </p>
                    ) : (
                      <p className="text-xs text-[var(--ink-tertiary)]">
                        Updated{" "}
                        <span className="tabular-nums">
                          {new Date(comp.updated_at).toLocaleDateString([], {
                            month: "short",
                            day: "numeric",
                            year: "numeric",
                          })}
                        </span>
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  {canManage && (
                    <div
                      className="flex items-center gap-1 pr-2 border-r border-[var(--card-line)]"
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                    >
                      <button
                        type="button"
                        aria-label={`Edit ${comp.name}`}
                        title="Edit competition"
                        onClick={() => openEditDialog(comp)}
                        className="flex h-8 w-8 items-center justify-center rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid)] border border-transparent hover:border-[var(--card-line)] transition-colors cursor-pointer"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>

                      <button
                        type="button"
                        aria-label={`Delete ${comp.name}`}
                        title="Delete competition"
                        onClick={() => {
                          setDeleteError(null);
                          setDeletingCompetition(comp);
                        }}
                        className="flex h-8 w-8 items-center justify-center rounded-[6px] text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] transition-colors cursor-pointer"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  )}

                  <span
                    aria-hidden="true"
                    className="flex h-8 w-8 items-center justify-center rounded-[8px] text-[var(--ink-tertiary)] transition-[color,transform] duration-150 group-hover:text-[var(--ink-primary)] group-hover:translate-x-0.5"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create / Edit Competition Dialog */}
      <Dialog
        open={dialogMode !== null}
        onClose={closeFormDialog}
        title={
          dialogMode === "create" ? "Create Competition" : "Edit Competition"
        }
        description={
          dialogMode === "create"
            ? `Add a competition inside ${displayEventName}.`
            : `Update metadata or slug for "${editingCompetition?.name}".`
        }
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={closeFormDialog}
              disabled={
                createCompetition.isPending || updateCompetition.isPending
              }
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={handleSaveCompetition}
              isLoading={
                createCompetition.isPending || updateCompetition.isPending
              }
            >
              {dialogMode === "create" ? "Create competition" : "Save changes"}
            </Button>
          </div>
        }
      >
        <form
          onSubmit={handleSaveCompetition}
          className="space-y-3 font-sans text-xs"
        >
          {formError && (
            <Alert variant="error" title="Could not save competition">
              {formError}
            </Alert>
          )}
          <Input
            label="Competition Name *"
            placeholder="e.g. AI & Robotics"
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
            disabled={
              createCompetition.isPending || updateCompetition.isPending
            }
            required
          />
          <div className="space-y-1">
            <Input
              label="Competition Slug *"
              placeholder="e.g. ai-robotics"
              value={slug}
              onChange={(e) => {
                setSlugTouched(true);
                setSlug(e.target.value.toLowerCase());
                if (fieldErrors.slug) setFieldErrors((prev) => ({ ...prev, slug: undefined }));
              }}
              error={fieldErrors.slug}
              disabled={
                createCompetition.isPending || updateCompetition.isPending
              }
              required
            />
            <p className="text-[11px] text-[var(--ink-tertiary)]">
              2–64 lowercase letters, numbers, and single hyphens. Scopes{" "}
              <span className="font-mono">
                /e/{eventSlug}/c/{slug || "slug"}
              </span>
              .
            </p>
          </div>
          <Textarea
            label="Description"
            placeholder="Optional summary of this competition..."
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={
              createCompetition.isPending || updateCompetition.isPending
            }
          />
        </form>
      </Dialog>

      {/* Delete Competition Confirmation Dialog */}
      <Dialog
        open={Boolean(deletingCompetition)}
        onClose={() => {
          if (!deleteCompetition.isPending) {
            setDeletingCompetition(null);
            setDeleteError(null);
          }
        }}
        title="Delete Competition?"
        description={
          deletingCompetition
            ? `Permanently delete "${deletingCompetition.name}" (${deletingCompetition.slug}) and all of its teams, submissions, and judging rounds.`
            : ""
        }
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => {
                setDeletingCompetition(null);
                setDeleteError(null);
              }}
              disabled={deleteCompetition.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="sm"
              type="button"
              onClick={handleConfirmDelete}
              isLoading={deleteCompetition.isPending}
            >
              Delete competition
            </Button>
          </div>
        }
      >
        <div className="space-y-3 text-xs font-sans">
          {deleteError && (
            <Alert variant="error" title="Could not delete competition">
              {deleteError}
            </Alert>
          )}
          <p className="text-[var(--ink-secondary)]">
            This action is permanent and cannot be undone.
          </p>
        </div>
      </Dialog>
    </div>
  );
}
