"use client";

import React, { useState } from "react";
import { Edit2, ExternalLink, Layers } from "lucide-react";
import { useMySubmission, useUpdateMySubmission } from "@/hooks/use-judging";
import { useToast } from "@/providers/toast-provider";
import { apiErrorMessage } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog } from "@/components/ui/dialog";
import { SkeletonCard } from "@/components/ui/skeleton";

const FIELDS = [
  { key: "github", label: "GitHub repository URL", placeholder: "https://github.com/..." },
  { key: "demo", label: "Live demo / prototype URL", placeholder: "https://my-demo-app.com" },
  { key: "slides", label: "Slide deck / presentation URL", placeholder: "https://slides.google.com/..." },
] as const;

/** A team's project submission. Independent of judging rounds; open whenever the team exists. */
export function SubmissionView() {
  const toast = useToast();
  const { data, isLoading } = useMySubmission();
  const update = useUpdateMySubmission();
  const [open, setOpen] = useState(false);
  const [project, setProject] = useState("");
  const [links, setLinks] = useState<Record<string, string>>({});

  if (isLoading) return <SkeletonCard />;
  // Only teams cleared by the attendance gate see the form; everyone else sees nothing.
  if (!data || !data.has_team || !data.can_submit) return null;

  const saved = FIELDS.filter((f) => data.links[f.key]);
  const blocked = !data.can_submit;

  const openEditor = () => {
    setProject(data.project_name || "");
    setLinks({ ...data.links });
    setOpen(true);
  };

  const save = async () => {
    try {
      await update.mutateAsync({ project_name: project.trim(), links });
      toast.success("Submission saved.");
      setOpen(false);
    } catch (err: unknown) {
      toast.error(apiErrorMessage(err, "Failed to save submission."));
    }
  };

  return (
    <div className="rounded-[16px] border border-[var(--card-line)] bg-[var(--card-solid)] p-4 sm:p-5 space-y-4 font-sans">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ink-primary)]">
          <Layers className="h-4 w-4 text-[var(--ink-tertiary)]" />
          Project submission
        </div>
        <Button variant="outline" size="sm" onClick={openEditor} disabled={blocked} className="text-xs flex items-center gap-1.5">
          <Edit2 className="h-3 w-3" />
          {data.project_name || saved.length > 0 ? "Edit" : "Submit"}
        </Button>
      </div>

      {blocked && (
        <p className="text-xs rounded-[10px] border border-[var(--color-coral-border)] bg-[var(--color-coral-dim)] p-3 text-[var(--ink-primary)]">
          {data.submit_blocked_reason || "Submissions are closed for your team."}
        </p>
      )}

      {data.project_name && (
        <div className="text-xs text-[var(--ink-secondary)]">
          Project: <strong className="text-[var(--ink-primary)]">{data.project_name}</strong>
        </div>
      )}

      {saved.length > 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
          {saved.map((f) => (
            <a
              key={f.key}
              href={data.links[f.key]}
              target="_blank"
              rel="noreferrer"
              className="p-3 rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] hover:bg-[var(--color-blue-dim)] transition-colors duration-150 flex items-center justify-between text-xs group"
            >
              <div className="truncate mr-2">
                <div className="font-semibold text-[var(--ink-primary)]">{f.key}</div>
                <div className="truncate text-[var(--ink-tertiary)] mt-0.5">{data.links[f.key]}</div>
              </div>
              <ExternalLink className="h-3.5 w-3.5 shrink-0 text-[var(--ink-tertiary)]" />
            </a>
          ))}
        </div>
      ) : (
        !blocked && (
          <p className="text-xs text-[var(--ink-secondary)]">
            Submit your project name and repository link for {data.team_name}.
          </p>
        )
      )}

      <Dialog open={open} onClose={() => setOpen(false)} title="Project submission" description="Update this any time while submissions are open.">
        <div className="space-y-4 pt-2 font-sans">
          <div className="space-y-1">
            <label className="block text-xs font-semibold text-[var(--ink-secondary)]">Project name</label>
            <Input value={project} maxLength={150} placeholder="e.g. SmartCampus" onChange={(e) => setProject(e.target.value)} className="text-xs" />
          </div>
          {FIELDS.map((f) => (
            <div key={f.key} className="space-y-1">
              <label className="block text-xs font-semibold text-[var(--ink-secondary)]">{f.label}</label>
              <Input
                value={links[f.key] || ""}
                placeholder={f.placeholder}
                onChange={(e) => setLinks((prev) => ({ ...prev, [f.key]: e.target.value }))}
                className="text-xs font-mono"
              />
            </div>
          ))}
          <div className="flex items-center justify-end gap-2 pt-3 border-t border-[var(--card-line)]">
            <Button variant="outline" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
            <Button size="sm" variant="primary" onClick={save} disabled={update.isPending}>Save</Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
