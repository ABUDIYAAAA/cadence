"use client";

import React, { useState } from "react";
import { Download, Globe, Plus } from "lucide-react";
import {
  useCreateScanSession,
  useScanSessions,
  useSessionReport,
  useSetScanSessionOpen,
  useTicketSettings,
  useUpdateTicketSettings,
} from "@/hooks/use-tickets";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { downloadCsv, slugify, toCsv } from "@/lib/csv";
import type { ScanKind, ScanSession } from "@/types/tickets";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { EmptyState } from "@/components/ui/empty-state";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const card = "bg-[var(--card-solid)] border border-[var(--card-line)] rounded-[16px] p-4 sm:p-5 space-y-3";

function InternalDomainsCard() {
  const toast = useToast();
  const { data, isLoading } = useTicketSettings();
  const update = useUpdateTicketSettings();
  // null = untouched, so the field mirrors the server until the admin edits it
  const [draft, setDraft] = useState<string | null>(null);

  if (isLoading || !data) return <SkeletonCard />;

  const text = draft ?? data.internal_email_domains.join(", ");
  const dirty = draft !== null && draft !== data.internal_email_domains.join(", ");

  const save = async () => {
    const domains = text.split(/[\s,;]+/).filter(Boolean);
    try {
      const saved = await update.mutateAsync(domains);
      setDraft(null);
      toast.success(
        saved.internal_email_domains.length
          ? `Internal domains: ${saved.internal_email_domains.join(", ")}`
          : "No internal domains: everyone counts as external.",
        "Saved"
      );
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to save domains"), "Error");
    }
  };

  return (
    <div className={card}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ink-primary)]">
          <Globe className="h-4 w-4 text-[var(--color-blue)]" />
          Internal email domains
        </div>
        <Badge variant="subtle" size="sm">
          {data.source === "database" ? "Saved in app" : "From server config"}
        </Badge>
      </div>
      <p className="text-xs text-[var(--ink-secondary)]">
        Attendees on these domains are internal: they see their QR on the dashboard and don&apos;t exit-scan. Everyone
        else is external: emailed their QR and the only ones who exit-scan. Separate several with commas.
      </p>
      <div className="flex gap-2 max-w-xl">
        <Input
          placeholder="e.g. college.edu, cs.college.edu"
          value={text}
          onChange={(e) => setDraft(e.target.value)}
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
          Save
        </Button>
      </div>
      {data.internal_email_domains.length === 0 && (
        <p className="text-xs text-[var(--color-coral)]">
          No internal domains set, so every attendee is treated as external.
        </p>
      )}
    </div>
  );
}

function SessionReportDialog({ session, onClose }: { session: ScanSession | null; onClose: () => void }) {
  const { data, isLoading } = useSessionReport(session?.id ?? null);

  const exportMissing = () => {
    if (!data || !session) return;
    const csv = toCsv(
      ["Name", "Email", "Phone", "Team", "Type"],
      data.missing.map((p) => [p.name, p.email, p.phone_number, p.team_name, p.is_external ? "External" : "Internal"])
    );
    downloadCsv(`${slugify(session.name)}-not-scanned.csv`, csv);
  };

  return (
    <Dialog
      open={Boolean(session)}
      onClose={onClose}
      title={session ? `${session.name} · report` : "Report"}
      description={
        session?.kind === "exit"
          ? "External attendees who entered. Those not scanned out are likely still inside."
          : "Everyone who entered. Missing means not yet scanned in this round."
      }
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={exportMissing}
            disabled={!data || data.missing.length === 0}
            className="gap-1.5 text-xs"
          >
            <Download className="h-3.5 w-3.5" />
            Export missing
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
              ["Scanned", data.scanned],
              ["Missing", data.missing.length],
            ].map(([label, value]) => (
              <div key={label} className="rounded-[12px] border border-[var(--card-line)] p-3">
                <div className="text-[var(--ink-secondary)]">{label}</div>
                <div className="text-xl font-bold font-mono text-[var(--ink-primary)]">{value}</div>
              </div>
            ))}
          </div>

          {data.missing.length === 0 ? (
            <div className="text-[var(--ink-secondary)]">Everyone expected has been scanned.</div>
          ) : (
            <div className="max-h-72 overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Not scanned</TableHead>
                    <TableHead>Team</TableHead>
                    <TableHead>Phone</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.missing.map((p) => (
                    <TableRow key={p.user_id}>
                      <TableCell className="font-semibold text-[var(--ink-primary)]">
                        {p.name}
                        <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">{p.email}</div>
                      </TableCell>
                      <TableCell>{p.team_name || "—"}</TableCell>
                      <TableCell className="font-mono">{p.phone_number || "—"}</TableCell>
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

/** Admin: internal domains, plus creating and monitoring attendance and exit scan sessions. */
export function ScanSessionsPanel() {
  const toast = useToast();
  const { data: sessions, isLoading } = useScanSessions();
  const create = useCreateScanSession();
  const setOpen = useSetScanSessionOpen();

  const [name, setName] = useState("");
  const [kind, setKind] = useState<ScanKind>("attendance");
  const [markTeam, setMarkTeam] = useState(true);
  const [reportFor, setReportFor] = useState<ScanSession | null>(null);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await create.mutateAsync({
        name: name.trim(),
        kind,
        ...(kind === "attendance" ? { mark_team: markTeam } : {}),
      });
      setName("");
      toast.success("Session created and open for scanning.", "Session ready");
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

  return (
    <div className="space-y-6 font-sans">
      <InternalDomainsCard />

      <form onSubmit={handleCreate} className={card}>
        <div className="text-sm font-semibold text-[var(--ink-primary)]">New scan session</div>
        <div className="flex flex-col sm:flex-row gap-2 sm:items-center">
          <Input
            placeholder="e.g. Night 1 attendance, Final exit"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="text-xs h-9 sm:max-w-xs"
          />
          <div className="w-full sm:w-44">
            <Select
              aria-label="Session type"
              value={kind}
              onChange={(e) => setKind(e.target.value as ScanKind)}
              className="text-xs h-9"
            >
              <option value="attendance">Attendance round</option>
              <option value="exit">Exit</option>
            </Select>
          </div>
          {kind === "attendance" && (
            <label className="flex items-center gap-2 text-xs cursor-pointer select-none text-[var(--ink-primary)]">
              <input
                type="checkbox"
                checked={markTeam}
                onChange={(e) => setMarkTeam(e.target.checked)}
                className="accent-[#83A2DB] h-4 w-4 rounded"
              />
              Team leader scan marks the whole team
            </label>
          )}
          <Button
            type="submit"
            variant="primary"
            size="sm"
            disabled={name.trim().length < 2 || create.isPending}
            isLoading={create.isPending}
            className="h-9 text-xs gap-1.5 sm:ml-auto"
          >
            <Plus className="h-3.5 w-3.5" />
            Create
          </Button>
        </div>
        <p className="text-[11px] text-[var(--ink-tertiary)]">
          Both types require the attendee to have checked in at entry. Exit is only for external attendees.
        </p>
      </form>

      <div className={card}>
        <div className="text-sm font-semibold text-[var(--ink-primary)]">Sessions</div>
        {isLoading ? (
          <SkeletonCard />
        ) : !sessions || sessions.length === 0 ? (
          <EmptyState title="No sessions yet" description="Create an attendance round or exit session above." />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead className="text-right">Scanned</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sessions.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-semibold text-[var(--ink-primary)]">
                    {s.name}
                    {s.kind === "attendance" && s.mark_team && (
                      <div className="text-[11px] font-normal text-[var(--ink-tertiary)]">Leader scan marks team</div>
                    )}
                  </TableCell>
                  <TableCell className="capitalize">{s.kind}</TableCell>
                  <TableCell className="text-right font-mono tabular-nums">{s.scanned_count}</TableCell>
                  <TableCell>
                    <Badge variant={s.is_open ? "emerald" : "subtle"} size="sm" dot>
                      {s.is_open ? "Open" : "Closed"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-2">
                      <Button variant="outline" size="sm" className="h-8 text-xs" onClick={() => setReportFor(s)}>
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
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <SessionReportDialog session={reportFor} onClose={() => setReportFor(null)} />
    </div>
  );
}
