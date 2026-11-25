"use client";

import React, { useEffect, useRef, useState, useCallback } from "react";
import jsQR from "jsqr";
import {
  Camera,
  Keyboard,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  QrCode,
  X,
} from "lucide-react";
import { useCheckIn, useSessionScan } from "@/hooks/use-tickets";
import {
  CheckInResponse,
  DuplicateCheckInResponse,
  ScanSession,
  TeamScanResult,
} from "@/types/tickets";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type ScanMode = "camera" | "manual";

interface TicketScannerProps {
  onScanSuccess?: (resp: CheckInResponse) => void;
  /** Scan into an attendance or exit session. Omit for the entry check-in. */
  session?: ScanSession | null;
}

const COPY = {
  entry: { title: "Gate Entry QR Scanner", approved: "Check-In Approved: Entry Granted", submit: "Verify & Check-In" },
  check_in: { title: "Gate Entry QR Scanner", approved: "Check-In Approved: Entry Granted", submit: "Verify & Check-In" },
  attendance: { title: "Attendance Scanner", approved: "Attendance Recorded", submit: "Verify & Mark Present" },
  exit: { title: "Exit Scanner", approved: "Exit Approved", submit: "Verify & Check-Out" },
} as const;

export function TicketScanner({ onScanSuccess, session }: TicketScannerProps) {
  const checkInMutation = useCheckIn();
  const sessionScanMutation = useSessionScan();
  const copy = COPY[session?.kind ?? "entry"];
  const isPending = checkInMutation.isPending || sessionScanMutation.isPending;
  const [lastTeam, setLastTeam] = useState<TeamScanResult | null>(null);
  const [ticketInput, setTicketInput] = useState("");
  // The camera is the default way in; typing or a USB wedge scanner is the alternative.
  const [mode, setMode] = useState<ScanMode>("camera");
  const [isCameraActive, setIsCameraActive] = useState(false);
  const [isStarting, setIsStarting] = useState(true);
  const [cameraAttempt, setCameraAttempt] = useState(0);
  const [cameraError, setCameraError] = useState<string | null>(null);

  const [lastSuccess, setLastSuccess] = useState<CheckInResponse | null>(null);
  const [lastDuplicate, setLastDuplicate] = useState<DuplicateCheckInResponse | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);
  const [scanSeq, setScanSeq] = useState(0);
  const [showOverlay, setShowOverlay] = useState(false);

  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const isProcessingRef = useRef(false);
  const overlayTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const triggerOverlay = useCallback(() => {
    setScanSeq((n) => n + 1);
    setShowOverlay(true);
    if (overlayTimerRef.current) clearTimeout(overlayTimerRef.current);
    overlayTimerRef.current = setTimeout(() => {
      setShowOverlay(false);
    }, 2500);
  }, []);

  const dismissResult = useCallback(() => {
    if (overlayTimerRef.current) {
      clearTimeout(overlayTimerRef.current);
      overlayTimerRef.current = null;
    }
    setShowOverlay(false);
    setLastSuccess(null);
    setLastDuplicate(null);
    setLastError(null);
    setLastTeam(null);
    isProcessingRef.current = false;
  }, []);

  useEffect(() => {
    return () => {
      if (overlayTimerRef.current) clearTimeout(overlayTimerRef.current);
    };
  }, []);

  const processTicketCode = useCallback(async (rawCode: string) => {
    const code = rawCode.trim();
    if (!code) return;

    setLastSuccess(null);
    setLastDuplicate(null);
    setLastError(null);
    setLastTeam(null);

    try {
      if (session) {
        const res = await sessionScanMutation.mutateAsync({ sessionId: session.id, ticketCode: code });
        setLastSuccess({
          success: true,
          message: res.message,
          ticket_code: code,
          checked_in_at: res.scanned_at,
          attendee: res.attendee,
        });
        setLastTeam(res.team ?? null);
      } else {
        const res = await checkInMutation.mutateAsync({ ticket_code: code });
        setLastSuccess(res);
        onScanSuccess?.(res);
      }
      setTicketInput("");
      triggerOverlay();
    } catch (err: unknown) {
      if (err && typeof err === "object") {
        const errorObj = err as { status?: number; error?: string; body?: Record<string, unknown> };
        const body = (errorObj.body ?? {}) as {
          code?: string;
          checked_in_at?: string;
          checked_in_by_name?: string;
          scanned_at?: string;
          scanned_by_name?: string;
          attendee?: CheckInResponse["attendee"];
        };
        const attendee = body.attendee ?? { id: "", name: "Attendee", email: "", role: "participant" };

        // A repeat scan: the same banner for entry and session scans
        if (body.code === "duplicate" || (!session && (errorObj.status === 409 || errorObj.error?.includes("already")))) {
          setLastDuplicate({
            error: errorObj.error || "Ticket has already been scanned",
            checked_in_at: body.scanned_at || body.checked_in_at || new Date().toISOString(),
            checked_in_by_name: body.scanned_by_name || body.checked_in_by_name,
            attendee,
          });
          triggerOverlay();
          return;
        }

        // Rule refusals name the person so the volunteer knows who was turned away
        const who = attendee.name && attendee.name !== "Attendee" ? ` (${attendee.name})` : "";
        setLastError((errorObj.error || "Scan failed. Please verify the ticket code.") + who);
      } else {
        setLastError("Scan failed. Please verify the ticket code.");
      }
      triggerOverlay();
    }
  }, [checkInMutation, sessionScanMutation, onScanSuccess, session, triggerOverlay]);

  // Stop camera stream cleanly
  const stopCamera = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
    setIsCameraActive(false);
  }, []);

  // Open the camera whenever camera mode is on (including on first load); release it otherwise.
  useEffect(() => {
    if (mode !== "camera") return;
    let cancelled = false;

    const open = async () => {
      if (!navigator.mediaDevices?.getUserMedia) {
        setCameraError("Camera access needs a secure (HTTPS) connection and a supported browser.");
        setIsStarting(false);
        return;
      }

      let stream: MediaStream | null = null;
      try {
        // Prefer the back camera at 720p for crisp scanning on larger viewports
        stream = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: { ideal: "environment" },
            width: { ideal: 1280 },
            height: { ideal: 720 },
          },
        });
      } catch {
        try {
          // Fall back to any available camera (FaceTime, webcam, etc.)
          stream = await navigator.mediaDevices.getUserMedia({ video: true });
        } catch (fallbackErr) {
          if (!cancelled) {
            setCameraError(
              fallbackErr instanceof Error
                ? fallbackErr.message
                : "Camera permission denied or no camera device found."
            );
            setIsStarting(false);
          }
          return;
        }
      }

      // Switched to manual entry (or unmounted) while the permission prompt was open
      if (cancelled) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }

      streamRef.current = stream;
      setIsCameraActive(true);
      setIsStarting(false);
    };

    open();
    return () => {
      cancelled = true;
      stopCamera();
    };
  }, [mode, cameraAttempt, stopCamera]);

  const selectMode = (next: ScanMode) => {
    if (next === mode) return;
    setCameraError(null);
    setIsStarting(next === "camera");
    setMode(next);
  };

  const retryCamera = () => {
    setCameraError(null);
    setIsStarting(true);
    setCameraAttempt((n) => n + 1);
  };

  // Sync stream to video element whenever camera becomes active
  useEffect(() => {
    if (isCameraActive && videoRef.current && streamRef.current) {
      if (videoRef.current.srcObject !== streamRef.current) {
        videoRef.current.srcObject = streamRef.current;
        videoRef.current.play().catch(() => {});
      }
    }
  }, [isCameraActive]);

  // Continuous frame scanning using jsQR
  useEffect(() => {
    if (!isCameraActive) return;

    let animId: number;
    let lastScanTime = 0;

    const scanFrame = () => {
      const video = videoRef.current;
      const canvas = canvasRef.current;

      if (
        video &&
        canvas &&
        video.readyState === video.HAVE_ENOUGH_DATA &&
        !isProcessingRef.current
      ) {
        const now = Date.now();
        // Throttle frame processing to once every 120ms to prevent CPU overload
        if (now - lastScanTime >= 120) {
          lastScanTime = now;

          const width = video.videoWidth;
          const height = video.videoHeight;

          if (width > 0 && height > 0) {
            canvas.width = width;
            canvas.height = height;

            const ctx = canvas.getContext("2d", { willReadFrequently: true });
            if (ctx) {
              ctx.drawImage(video, 0, 0, width, height);
              const imageData = ctx.getImageData(0, 0, width, height);

              const qr = jsQR(imageData.data, imageData.width, imageData.height, {
                inversionAttempts: "dontInvert",
              });

              if (qr && qr.data && qr.data.trim()) {
                isProcessingRef.current = true;
                processTicketCode(qr.data.trim()).finally(() => {
                  // Wait 2.5s before reading next QR to avoid double scanning
                  setTimeout(() => {
                    isProcessingRef.current = false;
                  }, 2500);
                });
              }
            }
          }
        }
      }

      animId = requestAnimationFrame(scanFrame);
    };

    animId = requestAnimationFrame(scanFrame);

    return () => {
      cancelAnimationFrame(animId);
    };
  }, [isCameraActive, processTicketCode]);

  const handleManualSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    processTicketCode(ticketInput);
  };

  return (
    <div className="rounded-[20px] border border-[var(--card-border)] bg-[var(--card-solid)] p-5 shadow-[var(--shadow-card)] font-sans space-y-5">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-4 border-b border-[var(--card-line)]">
        <div className="flex items-center gap-2.5">
          <div className="h-9 w-9 rounded-full bg-[var(--color-blue-dim)] flex items-center justify-center text-[var(--color-blue)]">
            <QrCode className="h-5 w-5" />
          </div>
          <div>
            <h3 className="font-bold text-base text-[var(--ink-primary)]">
              {copy.title}
            </h3>
            <p className="text-xs text-[var(--ink-secondary)]">
              {session ? `${session.name} · ` : ""}Scan an attendee&apos;s QR pass, or type the ticket code
            </p>
          </div>
        </div>

        <div
          role="radiogroup"
          aria-label="Scan method"
          className="inline-flex rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-0.5"
        >
          {(
            [
              { id: "camera", label: "Camera", Icon: Camera },
              { id: "manual", label: "Enter code", Icon: Keyboard },
            ] as const
          ).map(({ id, label, Icon }) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={mode === id}
              onClick={() => selectMode(id)}
              className={`inline-flex h-8 items-center gap-1.5 rounded-[10px] px-3 text-xs font-semibold transition-[background-color,color,transform] duration-150 ease-out active:scale-[0.97] cursor-pointer ${
                mode === id
                  ? "bg-[var(--card-solid)] text-[var(--ink-primary)] shadow-xs"
                  : "text-[var(--ink-secondary)] hover:text-[var(--ink-primary)]"
              }`}
            >
              <Icon className="h-3.5 w-3.5" />
              <span>{label}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Camera Viewport */}
      {mode === "camera" && isStarting && (
        <div className="aspect-[4/3] sm:aspect-video w-full max-w-3xl mx-auto rounded-[20px] bg-[var(--card-solid-subtle)] border border-[var(--card-line)] flex items-center justify-center text-sm text-[var(--ink-secondary)]">
          Starting camera... allow access if your browser asks.
        </div>
      )}

      {mode === "camera" && isCameraActive && (
        <div
          className={`relative rounded-[20px] bg-[#10141A] aspect-[4/3] sm:aspect-video w-full max-w-3xl mx-auto flex items-center justify-center overflow-hidden transition-[box-shadow] duration-200 shadow-[0_12px_32px_rgba(16,20,26,0.28)] ${
            lastSuccess
              ? "ring-4 ring-[var(--color-emerald)]"
              : lastDuplicate
              ? "ring-4 ring-[var(--color-amber)]"
              : lastError
              ? "ring-4 ring-[var(--color-coral)]"
              : "ring-1 ring-[var(--card-border)]"
          }`}
        >
          <video
            ref={(el) => {
              videoRef.current = el;
              if (el && streamRef.current && el.srcObject !== streamRef.current) {
                el.srcObject = streamRef.current;
                el.play().catch(() => {});
              }
            }}
            autoPlay
            playsInline
            muted
            className="w-full h-full object-cover"
            onLoadedMetadata={() => {
              videoRef.current?.play().catch(() => {});
            }}
          />

          {/* Hidden Canvas for QR Extraction */}
          <canvas ref={canvasRef} className="hidden" />

          {/* Viewfinder Target Overlay */}
          <div className="absolute inset-0 pointer-events-none flex flex-col items-center justify-between p-4 sm:p-5">
            <span className="text-[11px] text-white/95 bg-black/65 backdrop-blur-md px-3.5 py-1 rounded-full uppercase tracking-wider font-semibold">
              Point at attendee QR pass
            </span>

            {/* Corner-bracket reticle sized for comfortable framing */}
            <div className="relative w-52 h-52 sm:w-64 sm:h-64 flex items-center justify-center">
              <span className="absolute top-0 left-0 h-7 w-7 border-t-[3px] border-l-[3px] border-white/90 rounded-tl-[12px]" />
              <span className="absolute top-0 right-0 h-7 w-7 border-t-[3px] border-r-[3px] border-white/90 rounded-tr-[12px]" />
              <span className="absolute bottom-0 left-0 h-7 w-7 border-b-[3px] border-l-[3px] border-white/90 rounded-bl-[12px]" />
              <span className="absolute bottom-0 right-0 h-7 w-7 border-b-[3px] border-r-[3px] border-white/90 rounded-br-[12px]" />
              {!showOverlay && (
                <div className="w-[88%] border-t-2 border-[var(--color-blue)]/80 animate-pulse motion-reduce:animate-none" />
              )}
            </div>

            <span className="text-[11px] text-white/85 bg-black/60 backdrop-blur-md px-3 py-1 rounded-full font-medium">
              {isPending ? "Verifying pass..." : "Ready · auto-detecting"}
            </span>
          </div>

          {/* High-contrast in-viewport acknowledgement stamp during scan cooldown */}
          {showOverlay && (lastSuccess || lastDuplicate || lastError) && (
            <button
              key={scanSeq}
              type="button"
              onClick={() => {
                setShowOverlay(false);
                isProcessingRef.current = false;
              }}
              aria-label="Dismiss scan overlay and scan next"
              className={`absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 p-6 text-center backdrop-blur-[3px] animate-in fade-in-0 zoom-in-95 duration-200 motion-reduce:animate-none cursor-pointer ${
                lastSuccess
                  ? "bg-emerald-950/80 text-white"
                  : lastDuplicate
                  ? "bg-amber-950/82 text-white"
                  : "bg-red-950/82 text-white"
              }`}
            >
              <div
                className={`grid h-16 w-16 place-items-center rounded-full shadow-lg ${
                  lastSuccess
                    ? "bg-[var(--color-emerald)] text-white"
                    : lastDuplicate
                    ? "bg-[var(--color-amber)] text-white"
                    : "bg-[var(--color-coral)] text-white"
                }`}
              >
                {lastSuccess ? (
                  <CheckCircle2 className="h-9 w-9" />
                ) : lastDuplicate ? (
                  <AlertTriangle className="h-9 w-9" />
                ) : (
                  <XCircle className="h-9 w-9" />
                )}
              </div>

              <div className="space-y-1 max-w-md">
                <p className="text-lg sm:text-xl font-bold tracking-tight">
                  {lastSuccess
                    ? copy.approved
                    : lastDuplicate
                    ? "Already Scanned"
                    : "Scan Rejected"}
                </p>
                <p className="text-sm sm:text-base font-medium text-white/90">
                  {lastSuccess
                    ? lastSuccess.attendee.name
                    : lastDuplicate
                    ? lastDuplicate.attendee.name
                    : lastError}
                </p>
              </div>

              <span className="text-[11px] font-medium text-white/70 bg-black/30 px-3 py-1 rounded-full">
                Tap to scan next immediately
              </span>
            </button>
          )}
        </div>
      )}

      {mode === "camera" && cameraError && (
        <div className="max-w-3xl mx-auto p-3.5 rounded-[14px] border border-[var(--color-coral-border)] bg-[var(--color-coral-dim)] text-xs text-[var(--color-coral)] space-y-2.5">
          <p>Camera unavailable: {cameraError}</p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" type="button" onClick={retryCamera} className="h-8 text-xs">
              Try again
            </Button>
            <Button variant="primary" size="sm" type="button" onClick={() => selectMode("manual")} className="h-8 text-xs">
              Enter code instead
            </Button>
          </div>
        </div>
      )}

      {mode === "camera" && !cameraError && (
        <p className="text-center text-xs text-[var(--ink-secondary)]">
          Can&apos;t scan?{" "}
          <button
            type="button"
            onClick={() => selectMode("manual")}
            className="font-semibold text-[var(--color-blue)] hover:underline cursor-pointer"
          >
            Enter the ticket code instead
          </button>
        </p>
      )}

      {/* Manual entry (also receives USB handheld scanners, which type the code and press Enter) */}
      {mode === "manual" && (
        <form onSubmit={handleManualSubmit} className="flex gap-2 max-w-3xl mx-auto">
          <Input
            placeholder="Paste or scan ticket code (e.g. cadence:tkt:...)"
            value={ticketInput}
            onChange={(e) => setTicketInput(e.target.value)}
            disabled={isPending}
            className="text-xs h-10 font-mono"
            autoFocus
          />
          <Button
            variant="primary"
            size="sm"
            type="submit"
            disabled={!ticketInput.trim() || isPending}
            isLoading={isPending}
            className="text-xs h-10 px-5 shrink-0 font-semibold"
          >
            {copy.submit}
          </Button>
        </form>
      )}

      {/* Prominent Scan Acknowledgement Card */}
      {lastSuccess && (
        <div
          key={`ok-${scanSeq}`}
          role="status"
          aria-live="assertive"
          className="max-w-3xl mx-auto rounded-[16px] border-2 border-[var(--color-emerald)] bg-[var(--color-emerald-dim)] p-4 sm:p-5 space-y-3.5 animate-in fade-in-0 zoom-in-95 duration-200 motion-reduce:animate-none"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-3.5 min-w-0">
              <div className="grid h-11 w-11 shrink-0 place-items-center rounded-[12px] bg-[var(--color-emerald)] text-white">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div className="min-w-0">
                <p className="text-sm sm:text-base font-bold text-[var(--color-emerald)] leading-tight">
                  {copy.approved}
                </p>
                <p className="text-base sm:text-lg font-bold text-[var(--ink-primary)] truncate mt-0.5">
                  {lastSuccess.attendee.name}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={dismissResult}
              aria-label="Dismiss scan result"
              className="grid h-8 w-8 shrink-0 place-items-center rounded-[8px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-x-4 gap-y-2 pt-3 border-t border-[var(--color-emerald-border)] text-xs">
            <div className="min-w-0">
              <span className="text-[var(--ink-secondary)]">Email: </span>
              <span className="font-mono text-[var(--ink-primary)] break-all">{lastSuccess.attendee.email}</span>
            </div>
            <div>
              <span className="text-[var(--ink-secondary)]">Role: </span>
              <span className="uppercase font-semibold text-[var(--ink-primary)]">{lastSuccess.attendee.role}</span>
            </div>
            <div>
              <span className="text-[var(--ink-secondary)]">Squad: </span>
              <span className="text-[var(--ink-primary)] font-medium">{lastSuccess.attendee.team_name || "— No Team"}</span>
            </div>
            <div className="sm:col-span-3 text-[11px] text-[var(--ink-secondary)] font-mono pt-1">
              {lastSuccess.ticket_code} · {session ? "Scanned" : "Checked in"} at{" "}
              {new Date(lastSuccess.checked_in_at).toLocaleTimeString()}
            </div>
          </div>

          {lastTeam && (
            <div className="pt-3 border-t border-[var(--color-emerald-border)] space-y-1 text-xs">
              <div className="text-[var(--ink-primary)]">
                Team <strong>{lastTeam.team_name}</strong>: {lastTeam.marked + 1} marked present
                {lastTeam.already_scanned > 0 && ` (${lastTeam.already_scanned} already scanned)`}
              </div>
              {lastTeam.skipped.length > 0 && (
                <div className="text-[var(--color-coral)] font-medium">
                  Not marked:{" "}
                  {lastTeam.skipped
                    .map((m) => `${m.name} (${m.reason === "not_checked_in" ? "not checked in" : "no pass"})`)
                    .join(", ")}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Duplicate Warning Conflict */}
      {lastDuplicate && (
        <div
          key={`dup-${scanSeq}`}
          role="status"
          aria-live="assertive"
          className="max-w-3xl mx-auto rounded-[16px] border-2 border-[var(--color-amber)] bg-[var(--color-amber-dim)] p-4 sm:p-5 space-y-3 animate-in fade-in-0 zoom-in-95 duration-200 motion-reduce:animate-none"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-3.5 min-w-0">
              <div className="grid h-11 w-11 shrink-0 place-items-center rounded-[12px] bg-[var(--color-amber)] text-white">
                <AlertTriangle className="h-6 w-6" />
              </div>
              <div className="min-w-0">
                <p className="text-sm sm:text-base font-bold text-[var(--color-amber)] leading-tight">
                  Duplicate Scan · Already Scanned
                </p>
                <p className="text-base sm:text-lg font-bold text-[var(--ink-primary)] truncate mt-0.5">
                  {lastDuplicate.attendee.name}
                  {lastDuplicate.attendee.email ? (
                    <span className="ml-2 text-xs font-mono font-normal text-[var(--ink-secondary)]">
                      {lastDuplicate.attendee.email}
                    </span>
                  ) : null}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={dismissResult}
              aria-label="Dismiss duplicate warning"
              className="grid h-8 w-8 shrink-0 place-items-center rounded-[8px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <p className="pt-3 border-t border-[var(--color-amber-border)] text-xs sm:text-sm text-[var(--ink-primary)]">
            Previously scanned on{" "}
            <strong>{new Date(lastDuplicate.checked_in_at).toLocaleString()}</strong>
            {lastDuplicate.checked_in_by_name && (
              <>
                {" "}by <strong>{lastDuplicate.checked_in_by_name}</strong>
              </>
            )}
            .
          </p>
        </div>
      )}

      {/* Invalid / Rejected Ticket Error */}
      {lastError && (
        <div
          key={`err-${scanSeq}`}
          role="alert"
          aria-live="assertive"
          className="max-w-3xl mx-auto rounded-[16px] border-2 border-[var(--color-coral)] bg-[var(--color-coral-dim)] p-4 sm:p-5 flex items-start justify-between gap-3 animate-in fade-in-0 zoom-in-95 duration-200 motion-reduce:animate-none"
        >
          <div className="flex items-center gap-3.5 min-w-0">
            <div className="grid h-11 w-11 shrink-0 place-items-center rounded-[12px] bg-[var(--color-coral)] text-white">
              <XCircle className="h-6 w-6" />
            </div>
            <div className="min-w-0">
              <p className="text-sm sm:text-base font-bold text-[var(--color-coral)] leading-tight">
                Scan Rejected
              </p>
              <p className="text-sm font-medium text-[var(--ink-primary)] mt-0.5">
                {lastError}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={dismissResult}
            aria-label="Dismiss scan error"
            className="grid h-8 w-8 shrink-0 place-items-center rounded-[8px] text-[var(--ink-secondary)] hover:text-[var(--ink-primary)] hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  );
}
