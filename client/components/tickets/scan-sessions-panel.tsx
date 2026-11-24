"use client";

import React, { useState } from "react";
import {
  Copy,
  Download,
  Globe,
  KeyRound,
  Mail,
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
  "bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-3";

function TicketSettingsCard() {
  const toast = useToast();
  const { data, isLoading } = useTicketSettings();
  const update = useUpdateTicketSettings();

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

  return (
    <div className={card}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ink-primary)]">
          <Globe className="h-4 w-4 text-[var(--color-blue)]" />
          <span>Internal email domains & ticket email settings</span>
        </div>
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
            {data.source === "database" ? "Saved in app" : "From server config"}
          </Badge>
        </div>
      </div>

      <p className="text-xs text-[var(--ink-secondary)]">
        Attendees on these domains are internal: they see their QR on the
        dashboard and don&apos;t exit-scan. Everyone else is external: emailed
        their QR and the only ones who exit-scan. Separate several with commas.
      </p>

      <div className="flex flex-col sm:flex-row gap-2 max-w-2xl">
        <Input
          placeholder="e.g. college.edu, cs.college.edu"
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
        <span>Release tickets to participant dashboards automatically</span>
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

/** Admin: internal domains, plus creating and monitoring check-in, attendance, and exit scan sessions. */
export function ScanSessionsPanel() {
  const toast = useToast();
  const { competitions, activeCompetition } = useTenant();
  const { data: sessions, isLoading } = useScanSessions();
  const create = useCreateScanSession();
  const setOpen = useSetScanSessionOpen();
  const generateCode = useGenerateSessionCode();
  const clearCode = useClearSessionCode();

  const [name, setName] = useState("");
  const [kind, setKind] = useState<ScanKind>("attendance");
  const [markTeam, setMarkTeam] = useState(true);
  const [selectedCompetitionId, setSelectedCompetitionId] =
    useState<string>("");
  const [generateInitialCode, setGenerateInitialCode] = useState(false);
  const [reportFor, setReportFor] = useState<ScanSession | null>(null);
  const [codeBusySessionId, setCodeBusySessionId] = useState<string | null>(
    null
  );

  // Resolve effective competition ID for session creation
  const effectiveCompetitionId =
    selectedCompetitionId ||
    (markTeam ? activeCompetition?.id || competitions[0]?.id || "" : "");

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (kind === "attendance" && markTeam && !effectiveCompetitionId) {
      toast.error(
        "Select a competition when team-leader scanning is enabled.",
        "Competition required"
      );
      return;
    }

    try {
      const created = await create.mutateAsync({
        name: name.trim(),
        kind,
        ...(kind === "attendance"
          ? {
              mark_team: markTeam,
              competition_id: effectiveCompetitionId || undefined,
            }
          : {}),
        ...(generateInitialCode ? { generate_code: true } : {}),
      });
      setName("");
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

  const handleGenerateOrRotateCode = async (s: ScanSession) => {
    setCodeBusySessionId(s.id);
    try {
      const updated = await generateCode.mutateAsync({ sessionId: s.id });
      toast.success(
        `Active check-in code for "${s.name}": ${updated.checkin_code}`,
        s.checkin_code ? "Code rotated" : "Code generated"
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

  return (
    <div className="space-y-6 font-sans">
      <TicketSettingsCard />

      <form onSubmit={handleCreate} className={card}>
        <div className="text-sm font-semibold text-[var(--ink-primary)]">
          New scan session
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5 items-center">
          <Input
            placeholder="e.g. Night 1 attendance, Final exit"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="text-xs h-9"
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
                (kind === "attendance" && markTeam && !effectiveCompetitionId)
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
            <span>Generate rotatable self-check-in code</span>
          </label>
        </div>

        <p className="text-[11px] text-[var(--ink-tertiary)]">
          Attendance and exit sessions require the attendee to have checked in
          at entry. Full-team-via-leader scanning links the session to the
          selected competition.
        </p>
      </form>

      <div className={card}>
        <div className="text-sm font-semibold text-[var(--ink-primary)]">
          Sessions
        </div>
        {isLoading ? (
          <SkeletonCard />
        ) : !sessions || sessions.length === 0 ? (
          <EmptyState
            title="No sessions yet"
            description="Create an attendance round or exit session above."
          />
        ) : (
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
              {sessions.map((s) => {
                const isCodeBusy = codeBusySessionId === s.id;
                return (
                  <TableRow key={s.id}>
                    <TableCell className="font-semibold text-[var(--ink-primary)]">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span>{s.name}</span>
                        {s.is_default && (
                          <Badge variant="blue" size="sm">
                            Default entry
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
                        <div className="inline-flex items-center gap-1.5">
                          <span className="font-mono text-xs font-semibold tracking-wider px-2 py-0.5 rounded-[6px] border border-[var(--color-blue-border)] bg-[var(--color-blue-dim)] text-[var(--color-blue)]">
                            {s.checkin_code}
                          </span>
                          <button
                            type="button"
                            onClick={() => copySessionCode(s.checkin_code!)}
                            title="Copy check-in code"
                            aria-label={`Copy code ${s.checkin_code}`}
                            className="p-1 rounded-[5px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] cursor-pointer"
                          >
                            <Copy className="h-3.5 w-3.5" />
                          </button>
                          <button
                            type="button"
                            disabled={isCodeBusy}
                            onClick={() => handleGenerateOrRotateCode(s)}
                            title="Rotate check-in code (invalidates old code)"
                            aria-label={`Rotate check-in code for ${s.name}`}
                            className="p-1 rounded-[5px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] disabled:opacity-50 cursor-pointer"
                          >
                            <RefreshCw
                              className={`h-3.5 w-3.5 ${
                                isCodeBusy ? "animate-spin" : ""
                              }`}
                            />
                          </button>
                          <button
                            type="button"
                            disabled={isCodeBusy}
                            onClick={() => handleClearCode(s)}
                            title="Disable check-in code"
                            aria-label={`Disable check-in code for ${s.name}`}
                            className="p-1 rounded-[5px] text-[var(--ink-tertiary)] hover:text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] disabled:opacity-50 cursor-pointer"
                          >
                            <X className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={isCodeBusy}
                          onClick={() => handleGenerateOrRotateCode(s)}
                          className="h-7 px-2.5 text-[11px] gap-1"
                        >
                          <KeyRound className="h-3 w-3" />
                          <span>Generate code</span>
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
        )}
      </div>

      <SessionReportDialog
        session={reportFor}
        onClose={() => setReportFor(null)}
      />
    </div>
  );
}
