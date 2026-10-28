"use client";

import React, { useState } from "react";
import {
  BulkCreateResponse,
  BulkCreateUsersRequest,
  CreateUserRequest,
} from "@/types/users";
import { UserRole } from "@/types/auth";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { Upload, CheckCircle2, AlertCircle } from "lucide-react";

interface UserBulkCreateDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: BulkCreateUsersRequest) => Promise<BulkCreateResponse>;
  isLoading: boolean;
}

const SAMPLE_CSV = `email,name,role,phone_number,team_name,is_leader,send_invite
alice@example.com,Alice Smith,participant,+15550101,BinaryHackers,true,true
bob@example.com,Bob Jones,participant,+15550102,BinaryHackers,false,true
charlie@example.com,Charlie Brown,judge,,Panel-A,false,false`;

export function UserBulkCreateDialog({
  open,
  onClose,
  onSubmit,
  isLoading,
}: UserBulkCreateDialogProps) {
  const [csvText, setCsvText] = useState("");
  const [parsedUsers, setParsedUsers] = useState<CreateUserRequest[]>([]);
  const [parseError, setParseError] = useState<string | null>(null);
  const [result, setResult] = useState<BulkCreateResponse | null>(null);

  const resetDialog = () => {
    setCsvText("");
    setParsedUsers([]);
    setParseError(null);
    setResult(null);
  };

  const handleClose = () => {
    resetDialog();
    onClose();
  };

  const parseCsv = (text: string) => {
    try {
      const lines = text
        .trim()
        .split("\n")
        .map((l) => l.trim())
        .filter(Boolean);

      if (lines.length < 2) {
        throw new Error("CSV must include a header line and at least one user record.");
      }

      const header = lines[0].toLowerCase().split(",").map((h) => h.trim());
      const emailIdx = header.indexOf("email");
      const nameIdx = header.indexOf("name");

      if (emailIdx === -1 || nameIdx === -1) {
        throw new Error("CSV header must include 'email' and 'name' columns.");
      }

      const roleIdx = header.indexOf("role");
      const phoneIdx = header.indexOf("phone_number");
      const teamIdx = header.indexOf("team_name");
      const leaderIdx = header.indexOf("is_leader");
      const inviteIdx = header.indexOf("send_invite");

      const users: CreateUserRequest[] = [];

      for (let i = 1; i < lines.length; i++) {
        const row = lines[i].split(",").map((c) => c.trim());
        const email = row[emailIdx] || "";
        const name = row[nameIdx] || "";

        if (!email || !name) {
          throw new Error(`Row ${i + 1}: email and name are required.`);
        }

        const role = (roleIdx !== -1 && row[roleIdx] ? row[roleIdx] : "participant") as UserRole;
        const phone = phoneIdx !== -1 && row[phoneIdx] ? row[phoneIdx] : undefined;
        const teamName = teamIdx !== -1 && row[teamIdx] ? row[teamIdx] : undefined;
        const isLeader =
          leaderIdx !== -1
            ? row[leaderIdx] === "true" || row[leaderIdx] === "1"
            : false;
        const sendInvite =
          inviteIdx !== -1
            ? row[inviteIdx] === "true" || row[inviteIdx] === "1"
            : true;

        users.push({
          email,
          name,
          role,
          phone_number: phone,
          team_name: teamName,
          is_leader: isLeader,
          send_invite: sendInvite,
        });
      }

      setParsedUsers(users);
      setParseError(null);
    } catch (err: unknown) {
      setParseError(err instanceof Error ? err.message : "Failed to parse CSV");
      setParsedUsers([]);
    }
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      setCsvText(content);
      parseCsv(content);
    };
    reader.readAsText(file);
  };

  const handleSubmit = async () => {
    if (parsedUsers.length === 0) return;
    try {
      const res = await onSubmit({ users: parsedUsers });
      setResult(res);
    } catch (err: unknown) {
      setParseError(err instanceof Error ? err.message : "Import failed");
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Bulk Import Users (CSV)"
      description="Batch provision accounts with automatic team generation and invitations."
      footer={
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClose}
            disabled={isLoading}
          >
            {result ? "Close" : "Cancel"}
          </Button>

          {!result && (
            <Button
              variant="primary"
              size="sm"
              disabled={parsedUsers.length === 0 || isLoading}
              onClick={handleSubmit}
              isLoading={isLoading}
            >
              Import {parsedUsers.length} Users
            </Button>
          )}
        </div>
      }
    >
      <div className="space-y-4 font-mono text-xs">
        {result ? (
          <div className="space-y-3">
            <Alert variant="info" title="BULK IMPORT COMPLETED">
              Processed {result.total_processed} accounts.
            </Alert>

            <div className="border border-neutral-300 divide-y divide-neutral-200 bg-neutral-50">
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Total Created</span>
                <strong className="text-black">{result.total_created}</strong>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Invites Dispatched</span>
                <strong className="text-black">{result.invites_dispatched}</strong>
              </div>
              <div className="flex justify-between p-2.5">
                <span className="text-neutral-600">Skipped Records</span>
                <span className="font-semibold text-neutral-800">
                  {result.skipped_count}
                </span>
              </div>
            </div>

            {result.skipped_errors && result.skipped_errors.length > 0 && (
              <div className="border border-neutral-300 p-2.5 bg-neutral-100 max-h-36 overflow-y-auto space-y-1">
                <span className="font-bold text-[11px] uppercase">Skipped Warnings:</span>
                {result.skipped_errors.map((msg, i) => (
                  <div key={i} className="text-[10px] text-neutral-700">
                    • {msg}
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-neutral-600 font-semibold uppercase">
                CSV Input Data
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setCsvText(SAMPLE_CSV);
                  parseCsv(SAMPLE_CSV);
                }}
                className="h-6 text-[10px] px-2"
              >
                Insert Sample Data
              </Button>
            </div>

            <textarea
              rows={6}
              value={csvText}
              onChange={(e) => {
                setCsvText(e.target.value);
                parseCsv(e.target.value);
              }}
              placeholder={SAMPLE_CSV}
              className="w-full border border-neutral-300 bg-white p-2.5 font-mono text-xs focus:border-black focus:outline-none"
            />

            <div className="flex items-center justify-between border-t border-neutral-200 pt-2">
              <label className="inline-flex items-center gap-1.5 text-xs cursor-pointer border border-neutral-300 px-3 py-1.5 bg-neutral-50 hover:bg-neutral-100">
                <Upload className="h-3.5 w-3.5" />
                <span>Upload CSV File</span>
                <input
                  type="file"
                  accept=".csv,text/csv"
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>

              {parsedUsers.length > 0 && (
                <div className="flex items-center gap-1 text-black font-semibold">
                  <CheckCircle2 className="h-4 w-4" />
                  <span>{parsedUsers.length} valid rows detected</span>
                </div>
              )}
            </div>

            {parseError && (
              <div className="flex items-center gap-1.5 text-neutral-900 border border-black p-2 bg-neutral-100 font-bold">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{parseError}</span>
              </div>
            )}
          </div>
        )}
      </div>
    </Dialog>
  );
}
