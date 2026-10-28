"use client";

import React, { useState } from "react";
import { AuditLogResponse } from "@/types/audit";
import { Dialog } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Copy, Check, Terminal } from "lucide-react";

interface AuditDetailDialogProps {
  log: AuditLogResponse | null;
  open: boolean;
  onClose: () => void;
}

export function AuditDetailDialog({
  log,
  open,
  onClose,
}: AuditDetailDialogProps) {
  const [copied, setCopied] = useState(false);

  if (!log) return null;

  const handleCopyJson = () => {
    navigator.clipboard.writeText(JSON.stringify(log, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const hasMetadata = log.metadata && Object.keys(log.metadata).length > 0;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Audit Event Inspector"
      description={`Record ID: ${log.id}`}
      footer={
        <div className="flex items-center justify-between w-full">
          <Button
            variant="outline"
            size="sm"
            onClick={handleCopyJson}
            className="text-xs gap-1.5"
          >
            {copied ? (
              <>
                <Check className="h-3.5 w-3.5" />
                <span>JSON Copied</span>
              </>
            ) : (
              <>
                <Copy className="h-3.5 w-3.5" />
                <span>Copy Raw JSON</span>
              </>
            )}
          </Button>

          <Button variant="primary" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
      }
    >
      <div className="space-y-4 font-sans text-xs max-h-[65vh] overflow-y-auto pr-1">
        {/* Core Attributes */}
        <div className="rounded-[16px] border border-[var(--card-line)] divide-y divide-[var(--card-line)] overflow-hidden bg-[var(--card-solid)]">
          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Action</span>
            <Badge variant="solid" size="sm">
              {log.action}
            </Badge>
          </div>

          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Timestamp</span>
            <span className="text-[var(--ink-primary)] font-semibold font-mono">
              {new Date(log.created_at).toLocaleString()}
            </span>
          </div>

          <div className="p-3 bg-[var(--card-solid-subtle)] space-y-1">
            <span className="text-[var(--ink-secondary)]">Description</span>
            <p className="text-[var(--ink-primary)] font-sans text-xs leading-relaxed">
              {log.description}
            </p>
          </div>

          {/* Actor Info */}
          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">Actor</span>
            <div className="text-right">
              <span className="font-bold text-[var(--ink-primary)]">
                {log.actor_email || "System"}
              </span>
              {log.actor_role && (
                <span className="ml-2 uppercase text-[10px] text-[var(--ink-secondary)] font-semibold">
                  ({log.actor_role})
                </span>
              )}
              {log.actor_id && (
                <div className="text-[10px] text-[var(--ink-tertiary)] font-mono">{log.actor_id}</div>
              )}
            </div>
          </div>

          {/* Target Info */}
          <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
            <span className="text-[var(--ink-secondary)]">Target</span>
            <div className="text-right">
              <span className="font-semibold uppercase text-[var(--ink-primary)]">
                {log.target_type}
              </span>
              {log.target_id && (
                <div className="text-[10px] text-[var(--ink-secondary)] font-mono">
                  {log.target_id}
                </div>
              )}
            </div>
          </div>

          {/* Network Info */}
          <div className="flex justify-between p-3">
            <span className="text-[var(--ink-secondary)]">IP Address</span>
            <span className="font-mono text-[var(--ink-primary)]">
              {log.ip_address || "—"}
            </span>
          </div>

          {log.device_id && (
            <div className="flex justify-between p-3 bg-[var(--card-solid-subtle)]">
              <span className="text-[var(--ink-secondary)]">Device ID</span>
              <span className="font-mono text-[11px] text-[var(--ink-secondary)] break-all">
                {log.device_id}
              </span>
            </div>
          )}

          {log.user_agent && (
            <div className="p-3 space-y-1">
              <span className="text-[var(--ink-secondary)]">User Agent</span>
              <p className="text-[11px] text-[var(--ink-secondary)] break-all leading-tight font-mono">
                {log.user_agent}
              </p>
            </div>
          )}
        </div>

        {/* Metadata Section */}
        {hasMetadata && (
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 text-[var(--ink-secondary)] font-semibold text-[11px] uppercase">
              <Terminal className="h-3.5 w-3.5 text-[var(--color-blue)]" />
              <span>Event Context & Metadata</span>
            </div>
            <pre className="rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] text-[var(--ink-primary)] p-3.5 text-[11px] overflow-x-auto leading-relaxed font-mono">
              {JSON.stringify(log.metadata, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </Dialog>
  );
}
