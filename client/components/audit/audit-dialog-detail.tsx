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
      <div className="space-y-4 font-mono text-xs max-h-[65vh] overflow-y-auto pr-1">
        {/* Core Attributes */}
        <div className="border border-neutral-300 divide-y divide-neutral-200">
          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Action</span>
            <Badge variant="solid" size="sm">
              {log.action}
            </Badge>
          </div>

          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Timestamp</span>
            <span className="text-black font-semibold">
              {new Date(log.created_at).toLocaleString()} ({log.created_at})
            </span>
          </div>

          <div className="p-2.5 bg-neutral-50 space-y-1">
            <span className="text-neutral-500">Description</span>
            <p className="text-black font-sans sm:font-mono text-xs leading-relaxed">
              {log.description}
            </p>
          </div>

          {/* Actor Info */}
          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">Actor</span>
            <div className="text-right">
              <span className="font-bold text-black">
                {log.actor_email || "System"}
              </span>
              {log.actor_role && (
                <span className="ml-2 uppercase text-[10px] text-neutral-500 font-semibold">
                  ({log.actor_role})
                </span>
              )}
              {log.actor_id && (
                <div className="text-[10px] text-neutral-400">{log.actor_id}</div>
              )}
            </div>
          </div>

          {/* Target Info */}
          <div className="flex justify-between p-2.5 bg-neutral-50">
            <span className="text-neutral-500">Target</span>
            <div className="text-right">
              <span className="font-semibold uppercase text-black">
                {log.target_type}
              </span>
              {log.target_id && (
                <div className="text-[10px] text-neutral-500 font-mono">
                  {log.target_id}
                </div>
              )}
            </div>
          </div>

          {/* Network Info */}
          <div className="flex justify-between p-2.5">
            <span className="text-neutral-500">IP Address</span>
            <span className="font-mono text-black">
              {log.ip_address || "—"}
            </span>
          </div>

          {log.device_id && (
            <div className="flex justify-between p-2.5 bg-neutral-50">
              <span className="text-neutral-500">Device ID</span>
              <span className="font-mono text-neutral-700 break-all">
                {log.device_id}
              </span>
            </div>
          )}

          {log.user_agent && (
            <div className="p-2.5 space-y-1">
              <span className="text-neutral-500">User Agent</span>
              <p className="text-[11px] text-neutral-600 break-all leading-tight">
                {log.user_agent}
              </p>
            </div>
          )}
        </div>

        {/* Metadata Section */}
        {hasMetadata && (
          <div className="space-y-1.5">
            <div className="flex items-center gap-1.5 text-neutral-600 font-semibold text-[11px] uppercase">
              <Terminal className="h-3.5 w-3.5" />
              <span>Event Context & Metadata</span>
            </div>
            <pre className="border border-neutral-300 bg-neutral-900 text-neutral-100 p-3 text-[11px] overflow-x-auto leading-relaxed">
              {JSON.stringify(log.metadata, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </Dialog>
  );
}
