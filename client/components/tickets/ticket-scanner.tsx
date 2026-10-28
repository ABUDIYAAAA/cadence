"use client";

import React, { useEffect, useRef, useState, useCallback } from "react";
import jsQR from "jsqr";
import {
  Camera,
  CameraOff,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  QrCode,
  RotateCcw,
} from "lucide-react";
import { useCheckIn } from "@/hooks/use-tickets";
import { CheckInResponse, DuplicateCheckInResponse } from "@/types/tickets";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface TicketScannerProps {
  onScanSuccess?: (resp: CheckInResponse) => void;
}

export function TicketScanner({ onScanSuccess }: TicketScannerProps) {
  const checkInMutation = useCheckIn();
  const [ticketInput, setTicketInput] = useState("");
  const [isCameraActive, setIsCameraActive] = useState(false);
  const [cameraError, setCameraError] = useState<string | null>(null);

  const [lastSuccess, setLastSuccess] = useState<CheckInResponse | null>(null);
  const [lastDuplicate, setLastDuplicate] = useState<DuplicateCheckInResponse | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const isProcessingRef = useRef(false);

  const processTicketCode = useCallback(async (rawCode: string) => {
    const code = rawCode.trim();
    if (!code) return;

    setLastSuccess(null);
    setLastDuplicate(null);
    setLastError(null);

    try {
      const res = await checkInMutation.mutateAsync({ ticket_code: code });
      setLastSuccess(res);
      setTicketInput("");
      onScanSuccess?.(res);
    } catch (err: unknown) {
      if (err && typeof err === "object") {
        const errorObj = err as {
          status?: number;
          error?: string;
          checked_in_at?: string;
          checked_in_by_name?: string;
          attendee?: CheckInResponse["attendee"];
        };

        if (errorObj.status === 409 || errorObj.error?.includes("already")) {
          setLastDuplicate({
            error: errorObj.error || "Ticket has already been checked in",
            checked_in_at: errorObj.checked_in_at || new Date().toISOString(),
            checked_in_by_name: errorObj.checked_in_by_name,
            attendee: errorObj.attendee || {
              id: "",
              name: "Attendee",
              email: "",
              role: "participant",
            },
          });
          return;
        }

        setLastError(errorObj.error || "Check-in failed. Please verify the ticket code.");
      } else {
        setLastError("Check-in failed. Please verify the ticket code.");
      }
    }
  }, [checkInMutation, onScanSuccess]);

  // Start camera stream with multi-device fallback
  const startCamera = async () => {
    setCameraError(null);
    let stream: MediaStream | null = null;

    try {
      // 1. Try environment/back camera if available
      stream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: { ideal: "environment" },
          width: { ideal: 640 },
          height: { ideal: 480 },
        },
      });
    } catch {
      try {
        // 2. Fallback to any available video camera (FaceTime, webcam, etc.)
        stream = await navigator.mediaDevices.getUserMedia({ video: true });
      } catch (fallbackErr) {
        setCameraError(
          fallbackErr instanceof Error
            ? fallbackErr.message
            : "Camera permission denied or no camera device found."
        );
        setIsCameraActive(false);
        return;
      }
    }

    streamRef.current = stream;
    setIsCameraActive(true);

    // If video element is already rendered, attach stream immediately
    if (videoRef.current) {
      videoRef.current.srcObject = stream;
      videoRef.current.play().catch(() => {});
    }
  };

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

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      stopCamera();
    };
  }, [stopCamera]);

  const handleManualSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    processTicketCode(ticketInput);
  };

  return (
    <div className="border border-neutral-300 bg-white p-4 font-mono space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-3 border-b border-neutral-200">
        <div className="flex items-center gap-2">
          <QrCode className="h-5 w-5 text-black" />
          <h3 className="font-bold text-sm uppercase tracking-wider text-black">
            Gate Entry QR Scanner
          </h3>
        </div>

        <div className="flex items-center gap-2">
          {isCameraActive ? (
            <Button
              variant="outline"
              size="sm"
              onClick={stopCamera}
              className="text-xs gap-1.5 h-8"
            >
              <CameraOff className="h-3.5 w-3.5" />
              <span>Turn Camera Off</span>
            </Button>
          ) : (
            <Button
              variant="primary"
              size="sm"
              onClick={startCamera}
              className="text-xs gap-1.5 h-8"
            >
              <Camera className="h-3.5 w-3.5" />
              <span>Start Camera Scanner</span>
            </Button>
          )}
        </div>
      </div>

      {/* Camera Viewport */}
      {isCameraActive && (
        <div className="relative border-2 border-black bg-black aspect-video max-w-lg mx-auto flex items-center justify-center overflow-hidden">
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
          <div className="absolute inset-0 border-2 border-dashed border-white/70 m-6 pointer-events-none flex flex-col items-center justify-between p-2">
            <span className="text-[10px] text-white bg-black/80 px-2 py-0.5 uppercase tracking-widest font-bold">
              Point at attendee QR pass
            </span>
            <div className="w-full border-t border-red-500/80 animate-pulse my-auto" />
            <span className="text-[9px] text-white/80 bg-black/60 px-1.5 py-0.2">
              Auto-detecting...
            </span>
          </div>
        </div>
      )}

      {cameraError && (
        <div className="p-2 border border-black bg-neutral-100 text-xs text-neutral-800">
          ! {cameraError} (Use manual input or USB handheld scanner below)
        </div>
      )}

      {/* Manual & Barcode Scanner Wedge Form */}
      <form onSubmit={handleManualSubmit} className="flex gap-2 max-w-xl">
        <Input
          placeholder="Paste or scan ticket code (e.g. cadence:tkt:...)"
          value={ticketInput}
          onChange={(e) => setTicketInput(e.target.value)}
          disabled={checkInMutation.isPending}
          className="text-xs h-9 font-mono"
          autoFocus
        />
        <Button
          variant="primary"
          size="sm"
          type="submit"
          disabled={!ticketInput.trim() || checkInMutation.isPending}
          isLoading={checkInMutation.isPending}
          className="text-xs h-9 px-4 shrink-0"
        >
          Verify & Check-In
        </Button>
      </form>

      {/* Verification Feedback Banner */}
      {lastSuccess && (
        <div className="border-2 border-black p-4 bg-white space-y-2">
          <div className="flex items-center gap-2 text-black">
            <CheckCircle2 className="h-5 w-5 text-black shrink-0" />
            <span className="font-bold uppercase tracking-wider text-sm">
              Check-In Approved: Entry Granted
            </span>
          </div>

          <div className="border border-neutral-300 p-3 bg-neutral-50 grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
            <div>
              <span className="text-neutral-500">Attendee: </span>
              <strong className="text-black">{lastSuccess.attendee.name}</strong>
            </div>
            <div>
              <span className="text-neutral-500">Email: </span>
              <span className="font-mono">{lastSuccess.attendee.email}</span>
            </div>
            <div>
              <span className="text-neutral-500">System Role: </span>
              <span className="uppercase font-semibold">{lastSuccess.attendee.role}</span>
            </div>
            <div>
              <span className="text-neutral-500">Squad: </span>
              <span>{lastSuccess.attendee.team_name || "— No Team"}</span>
            </div>
            <div className="sm:col-span-2 text-[10px] text-neutral-400 font-mono pt-1 border-t border-neutral-200">
              Ticket Code: {lastSuccess.ticket_code} • Checked In:{" "}
              {new Date(lastSuccess.checked_in_at).toLocaleTimeString()}
            </div>
          </div>
        </div>
      )}

      {/* Duplicate Warning Conflict */}
      {lastDuplicate && (
        <div className="border-2 border-black p-4 bg-neutral-100 space-y-2">
          <div className="flex items-center gap-2 text-black">
            <AlertTriangle className="h-5 w-5 text-black shrink-0" />
            <span className="font-bold uppercase tracking-wider text-sm">
              Duplicate Scan Detected: Ticket Already Used!
            </span>
          </div>

          <div className="border border-neutral-400 p-3 bg-white space-y-1 text-xs">
            <p className="text-neutral-700">
              This ticket was already checked in on{" "}
              <strong>{new Date(lastDuplicate.checked_in_at).toLocaleString()}</strong>
              {lastDuplicate.checked_in_by_name && (
                <> by volunteer: <strong>{lastDuplicate.checked_in_by_name}</strong></>
              )}.
            </p>
            <div className="pt-1 text-neutral-800">
              <span>Registered to: </span>
              <strong>{lastDuplicate.attendee.name}</strong> ({lastDuplicate.attendee.email})
            </div>
          </div>
        </div>
      )}

      {/* Invalid Ticket Error */}
      {lastError && (
        <div className="border-2 border-black p-3 bg-neutral-50 flex items-center justify-between text-xs">
          <div className="flex items-center gap-2">
            <XCircle className="h-4 w-4 text-black shrink-0" />
            <span className="font-semibold text-black">{lastError}</span>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setLastError(null)}
            className="h-6 w-6 p-0"
          >
            <RotateCcw className="h-3.5 w-3.5" />
          </Button>
        </div>
      )}
    </div>
  );
}
