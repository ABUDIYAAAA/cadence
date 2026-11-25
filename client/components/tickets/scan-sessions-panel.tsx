"use client";

import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  ChevronDown,
  ChevronRight,
  Copy,
  Download,
  Globe,
  KeyRound,
  Mail,
  MoreHorizontal,
  Plus,
  RefreshCw,
  X,
} from "lucide-react";
import {
  useClearSessionCode,
  useCreateScanSession,
  useGenerateSessionCode,
  useScanSessions,
  useSessionReport,
  useSetScanSessionOpen,
  useTicketSettings,
  useUpdateTicketSettings,
} from "@/hooks/use-tickets";
import { useTenant } from "@/providers/tenant-provider";
import { useToast } from "@/providers/toast-provider";
import {
  buildCreateScanSessionPayload,
  normalizeCheckInCode,
  validateCustomCheckInCode,
} from "@/lib/tickets-utils";
import { apiErrorMessage } from "@/lib/utils";
import { downloadCsv, slugify, toCsv } from "@/lib/csv";
import type { ScanKind, ScanSession } from "@/types/tickets";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonCard } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const card =
  "bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-4";

function CodeActionsMenu({
  session,
  isBusy,
  onRotate,
  onEditCustom,
  onClear,
}: {
  session: ScanSession;
  isBusy: boolean;
  onRotate: () => void;
  onEditCustom: () => void;
  onClear: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [menuPos, setMenuPos] = useState<{
    top: number;
    left: number;
    openUp: boolean;
  } | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  const updatePosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const menuW = 176;
    const menuH = 124;
    const spaceBelow = window.innerHeight - rect.bottom - 8;
    const spaceAbove = rect.top - 8;
    const openUp = spaceBelow < menuH && spaceAbove > spaceBelow;
    const left = Math.min(
      Math.max(8, rect.left),
      window.innerWidth - menuW - 8
    );
    setMenuPos({
      top: openUp ? rect.top - 4 : rect.bottom + 4,
      left,
      openUp,
    });
  }, []);

  useLayoutEffect(() => {
    if (!open) return;
    updatePosition();
  }, [open, updatePosition]);

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node;
      if (
        triggerRef.current?.contains(target) ||
        menuRef.current?.contains(target)
      ) {
        return;
      }
      setOpen(false);
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleEscape);
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleEscape);
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [open, updatePosition]);

  return (
    <div className="relative">
      <button
        ref={triggerRef}
        type="button"
        disabled={isBusy}
        onClick={() => setOpen((prev) => !prev)}
        aria-expanded={open}
        aria-label={`Manage check-in code for ${session.name}`}
        title="Manage check-in code"
        className="p-1 rounded-[6px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] disabled:opacity-50 cursor-pointer"
      >
        <MoreHorizontal className="h-3.5 w-3.5" />
      </button>

      {open &&
        menuPos &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={menuRef}
            style={{
              position: "fixed",
              top: menuPos.top,
              left: menuPos.left,
              transform: menuPos.openUp ? "translateY(-100%)" : undefined,
            }}
            className="w-44 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] shadow-[var(--shadow-container)] p-1 z-[100] text-xs font-sans"
          >
            <button
              type="button"
              disabled={isBusy}
              onClick={() => {
                setOpen(false);
                onRotate();
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-[8px] text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] cursor-pointer"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${isBusy ? "animate-spin" : ""}`} />
              <span>Rotate random</span>
            </button>
            <button
              type="button"
              disabled={isBusy}
              onClick={() => {
                setOpen(false);
                onEditCustom();
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-[8px] text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] cursor-pointer"
            >
              <KeyRound className="h-3.5 w-3.5" />
              <span>Set custom code</span>
            </button>
            <button
              type="button"
              disabled={isBusy}
              onClick={() => {
                setOpen(false);
                onClear();
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-[8px] text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] cursor-pointer"
            >
              <X className="h-3.5 w-3.5" />
              <span>Disable code</span>
            </button>
          </div>,
          document.body
        )}
    </div>
  );
}

function TicketSettingsCard() {
  const toast = useToast();
  const { data, isLoading } = useTicketSettings();
  const update = useUpdateTicketSettings();

  const [expanded, setExpanded] = useState(false);
  const [domainsDraft, setDomainsDraft] = useState<string | null>(null);
  const [subjectDraft, setSubjectDraft] = useState<string | null>(null);
  const [htmlDraft, setHtmlDraft] = useState<string | null>(null);
  const [releasedDraft, setReleasedDraft] = useState<boolean | null>(null);
  const [showEmailTemplate, setShowEmailTemplate] = useState(false);

  if (isLoading || !data) return <SkeletonCard />;

  const serverDomains = (data.internal_email_domains ?? []).join(", ");
  const serverSubject = data.email_subject ?? "";
  const serverHtml = data.email_html ?? "";
  const serverReleased = Boolean(data.tickets_released);

  const domainsText = domainsDraft ?? serverDomains;
  const subjectText = subjectDraft ?? serverSubject;
  const htmlText = htmlDraft ?? serverHtml;
  const ticketsReleased = releasedDraft ?? serverReleased;

  const dirty =
    (domainsDraft !== null && domainsDraft !== serverDomains) ||
    (subjectDraft !== null && subjectDraft !== serverSubject) ||
    (htmlDraft !== null && htmlDraft !== serverHtml) ||
    (releasedDraft !== null && releasedDraft !== serverReleased);

  const save = async () => {
    const domains = domainsText.split(/[\s,;]+/).filter(Boolean);
    try {
      const saved = await update.mutateAsync({
        internal_email_domains: domains,
        excluded_domains: domains,
        email_subject: subjectText.trim(),
        email_html: htmlText,
        tickets_released: ticketsReleased,
      });
      setDomainsDraft(null);
      setSubjectDraft(null);
      setHtmlDraft(null);
      setReleasedDraft(null);
      toast.success(
        saved.internal_email_domains.length
          ? `Saved ticket settings (${saved.internal_email_domains.join(", ")})`
          : "Saved ticket settings (all attendees treated as external).",
        "Settings saved"
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to save settings"), "Error");
    }
  };

  const domainCount = data.internal_email_domains.length;
  const summaryText = [
    domainCount > 0
      ? `${domainCount} internal ${domainCount === 1 ? "domain" : "domains"}`
      : "All attendees external",
    serverReleased ? "Auto-release on" : "Auto-release off",
  ].join(" · ");

  return (
    <div className="bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded((prev) => !prev)}
        aria-expanded={expanded}
        className="w-full flex items-center justify-between gap-3 px-4 sm:px-5 py-3.5 text-left hover:bg-[var(--card-solid-subtle)]/50 transition-colors cursor-pointer"
      >
        <div className="flex items-center gap-2 min-w-0">
          {expanded ? (
            <ChevronDown className="h-4 w-4 text-[var(--ink-tertiary)] shrink-0" />
          ) : (
            <ChevronRight className="h-4 w-4 text-[var(--ink-tertiary)] shrink-0" />
          )}
          <Globe className="h-4 w-4 text-[var(--color-blue)] shrink-0" />
          <span className="text-sm font-semibold text-[var(--ink-primary)] truncate">
            Ticket & email settings
          </span>
          {dirty && (
            <Badge variant="amber" size="sm">
              Unsaved
            </Badge>
          )}
        </div>
        <span className="text-xs text-[var(--ink-secondary)] truncate hidden sm:inline">
          {summaryText}
        </span>
      </button>

      {expanded && (
        <div className="px-4 sm:px-5 pb-4 sm:pb-5 pt-3 border-t border-[var(--card-line)] space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-[var(--ink-secondary)] max-w-2xl">
              Attendees on internal domains see their QR on the dashboard and
              don&apos;t exit-scan. Everyone else is external and emailed their
              QR.
            </p>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setShowEmailTemplate((prev) => !prev)}
                className="h-7 px-2.5 text-xs gap-1.5"
              >
                <Mail className="h-3.5 w-3.5" />
                <span>
                  {showEmailTemplate ? "Hide email template" : "Email template"}
                </span>
              </Button>
              <Badge variant="subtle" size="sm">
                {data.source === "database"
                  ? "Saved in app"
                  : "From server config"}
              </Badge>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row gap-2 max-w-2xl">
            <Input
              placeholder="Internal domains, e.g. college.edu, cs.college.edu"
              value={domainsText}
              onChange={(e) => setDomainsDraft(e.target.value)}
              className="text-xs h-9 font-mono"
            />
            <Button
              variant="primary"
              size="sm"
              onClick={save}
              disabled={!dirty || update.isPending}
              isLoading={update.isPending}
              className="h-9 text-xs shrink-0"
            >
              Save settings
            </Button>
          </div>

          {data.internal_email_domains.length === 0 && (
            <p className="text-xs text-[var(--color-coral)]">
              No internal domains set, so every attendee is treated as external.
            </p>
          )}

          <label className="flex items-center gap-2 text-xs cursor-pointer select-none text-[var(--ink-primary)] pt-1">
            <input
              type="checkbox"
              checked={ticketsReleased}
              onChange={(e) => setReleasedDraft(e.target.checked)}
              className="accent-[var(--color-blue)] h-4 w-4 rounded"
            />
            <span>
              Release tickets to participant dashboards automatically
            </span>
          </label>

          {showEmailTemplate && (
            <div className="pt-3 border-t border-[var(--card-line)] space-y-3 max-w-2xl">
              <Input
                label="Default Ticket Email Subject (optional)"
                placeholder="e.g. Your Entry Pass for DevClub Hackathon"
                value={subjectText}
                onChange={(e) => setSubjectDraft(e.target.value)}
                className="text-xs"
              />
              <Textarea
                label="Default Custom Email HTML Body (optional, sanitized on server)"
                placeholder="Optional custom HTML message included in ticket emails..."
                rows={3}
                value={htmlText}
                onChange={(e) => setHtmlDraft(e.target.value)}
                className="text-xs font-mono"
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

type ReportView = "all" | "present" | "absent";

function SessionReportDialog({
  session,
  onClose,
}: {
  session: ScanSession | null;
  onClose: () => void;
}) {
  const { data, isLoading } = useSessionReport(session?.id ?? null);
  const [view, setView] = useState<ReportView>("all");

  const people = data?.people ?? [];
  const visible = people.filter((p) =>
    view === "all" ? true : view === "present" ? p.present : !p.present
  );

  const exportReport = () => {
    if (!data || !session) return;
    const csv = toCsv(
      ["Name", "Email", "Phone", "Team", "Type", "Status", "Scanned at"],
      people.map((p) => [
        p.name,
        p.email,
        p.phone_number,
        p.team_name,
        p.is_external ? "External" : "Internal",
        p.present ? "Present" : "Absent",
        p.scanned_at,
      ])
    );
    downloadCsv(`${slugify(session.name)}-attendance.csv`, csv);
  };

  return (
    <Dialog
      open={Boolean(session)}
      onClose={onClose}
      title={session ? `${session.name} · report` : "Report"}
      description={
        session?.kind === "exit"
          ? "External attendees who entered. Absent means not yet scanned out, so likely still inside."
          : "Everyone with a pass. Teams and members update live, so changes made during the session show on refresh."
      }
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={exportReport}
            disabled={!data || people.length === 0}
            className="gap-1.5 text-xs"
          >
            <Download className="h-3.5 w-3.5" />
            Export report
          </Button>
          <Button variant="outline" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
      }
    >
      {isLoading || !data ? (
        <SkeletonCard />
      ) : (
        <div className="space-y-4 text-xs">
          <div className="grid grid-cols-3 gap-2">
            {[
              ["Expected", data.eligible],
              ["Present", data.scanned],
              ["Absent", data.missing.length],
            ].map(([label, value]) => (
              <div
                key={label}
                className="rounded-[12px] border border-[var(--card-line)] p-3"
              >
                <div className="text-[var(--ink-secondary)]">{label}</div>
                <div className="text-xl font-bold font-mono text-[var(--ink-primary)]">
                  {value}
                </div>
              </div>
            ))}
          </div>

          <div className="flex gap-1.5">
            {(["all", "present", "absent"] as const).map((v) => (
              <Button
                key={v}
                variant={view === v ? "primary" : "outline"}
                size="sm"
                onClick={() => setView(v)}
                className="h-7 px-3 text-xs capitalize"
              >
                {v}
              </Button>
            ))}
          </div>

          {visible.length === 0 ? (
            <div className="text-[var(--ink-secondary)]">Nobody here.</div>
          ) : (
            <div className="max-h-72 overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Team</TableHead>
                    <TableHead>Phone</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {visible.map((p) => (
                    <TableRow key={p.user_id}>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">
                        {p.name}
                        <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">
                          {p.email}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={p.present ? "emerald" : "subtle"}
                          dot
                          size="sm"
                        >
                          {p.present ? "Present" : "Absent"}
                        </Badge>
                      </TableCell>
                      <TableCell>{p.team_name || "—"}</TableCell>
                      <TableCell className="font-mono">
                        {p.phone_number || "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}

/** Admin: creating and monitoring check-in, attendance, and exit scan sessions, plus ticket settings. */
export function ScanSessionsPanel() {
  const toast = useToast();
  const { competitions, activeCompetition } = useTenant();
  const { data: sessions = [], isLoading } = useScanSessions();
  const create = useCreateScanSession();
  const setOpen = useSetScanSessionOpen();
  const generateCode = useGenerateSessionCode();
  const clearCode = useClearSessionCode();

  // Collapsible UI state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [showClosed, setShowClosed] = useState(false);

  const [name, setName] = useState("");
  const [kind, setKind] = useState<ScanKind>("attendance");
  const [markTeam, setMarkTeam] = useState(true);
  const [selectedCompetitionId, setSelectedCompetitionId] =
    useState<string>("");
  const [generateInitialCode, setGenerateInitialCode] = useState(false);
  const [initialCustomCode, setInitialCustomCode] = useState("");
  const [editingCodeSession, setEditingCodeSession] =
    useState<ScanSession | null>(null);
  const [customCodeDraft, setCustomCodeDraft] = useState("");
  const [reportFor, setReportFor] = useState<ScanSession | null>(null);
  const [codeBusySessionId, setCodeBusySessionId] = useState<string | null>(
    null
  );

  const openSessions = useMemo(
    () => sessions.filter((s) => s.is_open),
    [sessions]
  );
  const closedSessions = useMemo(
    () => sessions.filter((s) => !s.is_open),
    [sessions]
  );

  // Resolve effective competition ID for session creation
  const effectiveCompetitionId =
    selectedCompetitionId ||
    (markTeam ? activeCompetition?.id || competitions[0]?.id || "" : "");

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const built = buildCreateScanSessionPayload({
      name,
      kind,
      markTeam,
      competitionId: effectiveCompetitionId,
      enableCode: generateInitialCode,
      customCode: initialCustomCode,
    });
    if (built.error !== undefined) {
      toast.error(built.error, "Invalid session configuration");
      return;
    }

    try {
      const created = await create.mutateAsync(built.payload);
      setName("");
      setInitialCustomCode("");
      setShowCreateForm(false);
      toast.success(
        created.checkin_code
          ? `Session created with check-in code ${created.checkin_code}.`
          : "Session created and open for scanning.",
        "Session ready"
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to create session"), "Error");
    }
  };

  const toggle = async (s: ScanSession) => {
    try {
      await setOpen.mutateAsync({ id: s.id, isOpen: !s.is_open });
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to update session"), "Error");
    }
  };

  const handleGenerateOrRotateCode = async (
    s: ScanSession,
    customCode?: string
  ) => {
    const codeErr = validateCustomCheckInCode(customCode);
    if (codeErr) {
      toast.error(codeErr, "Invalid code");
      return;
    }
    setCodeBusySessionId(s.id);
    try {
      const normalized = normalizeCheckInCode(customCode);
      const updated = await generateCode.mutateAsync({
        sessionId: s.id,
        code: normalized || undefined,
      });
      setEditingCodeSession(null);
      setCustomCodeDraft("");
      toast.success(
        `Active check-in code for "${s.name}": ${updated.checkin_code}`,
        s.checkin_code ? "Code updated" : "Code generated"
      );
    } catch (err: unknown) {
      toast.error(
        apiErrorMessage(err, "Failed to generate check-in code"),
        "Error"
      );
    } finally {
      setCodeBusySessionId(null);
    }
  };

  const handleClearCode = async (s: ScanSession) => {
    setCodeBusySessionId(s.id);
    try {
      await clearCode.mutateAsync(s.id);
      toast.info(
        `Self-check-in code disabled for "${s.name}".`,
        "Code cleared"
      );
    } catch (err: unknown) {
      toast.error(
        apiErrorMessage(err, "Failed to clear check-in code"),
        "Error"
      );
    } finally {
      setCodeBusySessionId(null);
    }
  };

  const copySessionCode = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success(`Check-in code ${code} copied.`, "Copied");
    } catch {
      toast.info(`Check-in code: ${code}`, "Copy manually");
    }
  };

  const renderTable = (rows: ScanSession[]) => (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Type & Scope</TableHead>
          <TableHead>Self Check-In Code</TableHead>
          <TableHead className="text-right">Scanned</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((s) => {
          const isCodeBusy = codeBusySessionId === s.id;
          return (
            <TableRow key={s.id}>
              <TableCell className="font-semibold text-[var(--ink-primary)]">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span>{s.name}</span>
                  {s.is_default && (
                    <Badge variant="blue" size="sm">
                      Default
                    </Badge>
                  )}
                </div>
                {s.kind === "attendance" && s.mark_team && (
                  <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">
                    Leader scan marks team
                  </div>
                )}
              </TableCell>

              <TableCell>
                <div className="flex flex-col items-start gap-1">
                  <span className="capitalize text-xs text-[var(--ink-primary)]">
                    {s.kind === "check_in" ? "Entry check-in" : s.kind}
                  </span>
                  {s.competition_name && (
                    <Badge variant="subtle" size="sm">
                      {s.competition_name}
                    </Badge>
                  )}
                </div>
              </TableCell>

              <TableCell>
                {s.checkin_code ? (
                  <div className="inline-flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => copySessionCode(s.checkin_code!)}
                      title="Copy check-in code"
                      className="inline-flex items-center gap-1.5 font-mono text-xs font-semibold tracking-wider px-2 py-0.5 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)] text-[var(--color-blue)] hover:opacity-90 cursor-pointer"
                    >
                      <span>{s.checkin_code}</span>
                      <Copy className="h-3 w-3" />
                    </button>
                    <CodeActionsMenu
                      session={s}
                      isBusy={isCodeBusy}
                      onRotate={() => handleGenerateOrRotateCode(s)}
                      onEditCustom={() => {
                        setEditingCodeSession(s);
                        setCustomCodeDraft(s.checkin_code || "");
                      }}
                      onClear={() => handleClearCode(s)}
                    />
                  </div>
                ) : (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={isCodeBusy}
                    onClick={() => {
                      setEditingCodeSession(s);
                      setCustomCodeDraft("");
                    }}
                    className="h-7 px-2.5 text-[11px] gap-1"
                  >
                    <KeyRound className="h-3 w-3" />
                    <span>Enable code</span>
                  </Button>
                )}
              </TableCell>

              <TableCell className="text-right font-mono tabular-nums">
                {s.scanned_count}
              </TableCell>

              <TableCell>
                <Badge
                  variant={s.is_open ? "emerald" : "subtle"}
                  size="sm"
                  dot
                >
                  {s.is_open ? "Open" : "Closed"}
                </Badge>
              </TableCell>

              <TableCell className="text-right">
                <div className="inline-flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8 text-xs"
                    onClick={() => setReportFor(s)}
                  >
                    Report
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8 text-xs"
                    onClick={() => toggle(s)}
                    disabled={setOpen.isPending}
                  >
                    {s.is_open ? "Close" : "Reopen"}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );

  return (
    <div className="space-y-5 font-sans">
      {/* Sessions Card (with collapsible Create form) */}
      <div className={card}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-[var(--ink-primary)]">
              Sessions
            </h2>
            <Badge variant="subtle" size="sm">
              {openSessions.length} open
            </Badge>
          </div>

          <Button
            type="button"
            variant={showCreateForm ? "outline" : "primary"}
            size="sm"
            onClick={() => setShowCreateForm((prev) => !prev)}
            className="h-8 px-3 text-xs gap-1.5"
          >
            {showCreateForm ? (
              <>
                <X className="h-3.5 w-3.5" />
                <span>Cancel</span>
              </>
            ) : (
              <>
                <Plus className="h-3.5 w-3.5" />
                <span>New session</span>
              </>
            )}
          </Button>
        </div>

        {/* Collapsible Create Session Form */}
        {showCreateForm && (
          <form
            onSubmit={handleCreate}
            className="rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)]/50 p-3.5 space-y-3"
          >
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5 items-center">
              <Input
                placeholder="e.g. Night 1 attendance, Final exit"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="text-xs h-9"
                autoFocus
              />

              <Select
                aria-label="Session type"
                value={kind}
                onChange={(e) => setKind(e.target.value as ScanKind)}
                className="text-xs h-9"
              >
                <option value="attendance">Attendance round</option>
                <option value="exit">Exit</option>
              </Select>

              {kind === "attendance" && (
                <Select
                  aria-label="Competition scope"
                  value={effectiveCompetitionId}
                  onChange={(e) => setSelectedCompetitionId(e.target.value)}
                  className="text-xs h-9"
                >
                  {!markTeam && (
                    <option value="">All event members (no competition)</option>
                  )}
                  {competitions.length === 0 ? (
                    <option value="" disabled>
                      No competitions available
                    </option>
                  ) : (
                    competitions.map((comp) => (
                      <option key={comp.id} value={comp.id}>
                        Competition: {comp.name}
                      </option>
                    ))
                  )}
                </Select>
              )}

              <div className="flex justify-end">
                <Button
                  type="submit"
                  variant="primary"
                  size="sm"
                  disabled={
                    name.trim().length < 2 ||
                    create.isPending ||
                    (kind === "attendance" &&
                      markTeam &&
                      !effectiveCompetitionId)
                  }
                  isLoading={create.isPending}
                  className="h-9 text-xs gap-1.5 w-full sm:w-auto"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Create session
                </Button>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-4 pt-1">
              {kind === "attendance" && (
                <label className="flex items-center gap-2 text-xs cursor-pointer select-none text-[var(--ink-primary)]">
                  <input
                    type="checkbox"
                    checked={markTeam}
                    onChange={(e) => setMarkTeam(e.target.checked)}
                    className="accent-[var(--color-blue)] h-4 w-4 rounded"
                  />
                  <span>Team leader scan marks the whole team</span>
                </label>
              )}

              <label className="flex items-center gap-2 text-xs cursor-pointer select-none text-[var(--ink-primary)]">
                <input
                  type="checkbox"
                  checked={generateInitialCode}
                  onChange={(e) => setGenerateInitialCode(e.target.checked)}
                  className="accent-[var(--color-blue)] h-4 w-4 rounded"
                />
                <span>Enable rotatable self-check-in code</span>
              </label>

              {generateInitialCode && (
                <div className="w-56">
                  <Input
                    placeholder="Custom code (blank = random)"
                    value={initialCustomCode}
                    maxLength={32}
                    onChange={(e) =>
                      setInitialCustomCode(e.target.value.toUpperCase())
                    }
                    className="h-8 text-xs font-mono uppercase tracking-wider"
                  />
                </div>
              )}
            </div>

            <p className="text-[11px] text-[var(--ink-tertiary)]">
              Attendance and exit sessions require the attendee to have checked
              in at entry. Full-team-via-leader scanning links the session to
              the selected competition.
            </p>
          </form>
        )}

        {/* Open & Closed Sessions */}
        {isLoading ? (
          <SkeletonCard />
        ) : sessions.length === 0 ? (
          <EmptyState
            title="No sessions yet"
            description="Create an attendance round or exit session to start scanning."
            action={
              !showCreateForm ? (
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  onClick={() => setShowCreateForm(true)}
                  className="text-xs gap-1.5"
                >
                  <Plus className="h-3.5 w-3.5" />
                  New session
                </Button>
              ) : undefined
            }
          />
        ) : (
          <div className="space-y-3">
            {openSessions.length > 0 ? (
              renderTable(openSessions)
            ) : (
              <p className="text-xs text-[var(--ink-secondary)]">
                No open sessions right now.
              </p>
            )}

            {closedSessions.length > 0 && (
              <div className="rounded-[12px] border border-[var(--card-line)] overflow-hidden">
                <button
                  type="button"
                  onClick={() => setShowClosed((prev) => !prev)}
                  aria-expanded={showClosed || openSessions.length === 0}
                  className="w-full flex items-center justify-between gap-3 px-3.5 py-2.5 text-left bg-[var(--card-solid-subtle)]/50 hover:bg-[var(--card-solid-subtle)] transition-colors cursor-pointer"
                >
                  <div className="flex items-center gap-2">
                    {showClosed || openSessions.length === 0 ? (
                      <ChevronDown className="h-4 w-4 text-[var(--ink-tertiary)]" />
                    ) : (
                      <ChevronRight className="h-4 w-4 text-[var(--ink-tertiary)]" />
                    )}
                    <span className="text-xs font-semibold text-[var(--ink-primary)]">
                      Closed sessions
                    </span>
                  </div>
                  <span className="text-[11px] text-[var(--ink-secondary)] tabular-nums">
                    {closedSessions.length} closed
                  </span>
                </button>
                {(showClosed || openSessions.length === 0) && (
                  <div className="border-t border-[var(--card-line)] bg-[var(--card-solid)]">
                    {renderTable(closedSessions)}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Collapsible Ticket & Email Settings */}
      <TicketSettingsCard />

      <SessionReportDialog
        session={reportFor}
        onClose={() => setReportFor(null)}
      />

      <Dialog
        open={Boolean(editingCodeSession)}
        onClose={() => {
          setEditingCodeSession(null);
          setCustomCodeDraft("");
        }}
        title={
          editingCodeSession
            ? `Check-in code for ${editingCodeSession.name}`
            : "Set check-in code"
        }
        description="Set a custom 2–32 character code (or leave blank to generate a random 6-character code). Setting a new code immediately invalidates any previous code on this session."
        footer={
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setEditingCodeSession(null);
                setCustomCodeDraft("");
              }}
              disabled={generateCode.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              isLoading={generateCode.isPending}
              onClick={() =>
                editingCodeSession &&
                handleGenerateOrRotateCode(editingCodeSession, customCodeDraft)
              }
            >
              {customCodeDraft.trim()
                ? "Save custom code"
                : "Generate random code"}
            </Button>
          </div>
        }
      >
        <div className="space-y-3 font-sans text-xs">
          <Input
            label="Self check-in code"
            placeholder="e.g. NIGHT1 or MAINDOOR (leave blank for random)"
            value={customCodeDraft}
            maxLength={32}
            onChange={(e) => setCustomCodeDraft(e.target.value.toUpperCase())}
            className="font-mono uppercase tracking-wider text-xs"
            autoFocus
          />
        </div>
      </Dialog>
    </div>
  );
}
