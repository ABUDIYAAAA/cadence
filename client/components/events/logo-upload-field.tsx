"use client";

import React, { useRef, useState } from "react";
import { ImagePlus, Trash2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";

const MAX_LOGO_BYTES = 1.5 * 1024 * 1024; // Fits comfortably under 2 MiB JSON limit

interface LogoUploadFieldProps {
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
}

/**
 * Allows uploading an event logo/photo (encoded as a data URL) with live preview and clear control.
 */
export function LogoUploadField({
  value,
  onChange,
  disabled = false,
}: LogoUploadFieldProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setError(null);
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;

    if (!file.type.startsWith("image/")) {
      setError("Please select an image file (PNG, JPG, WebP, or SVG).");
      return;
    }
    if (file.size > MAX_LOGO_BYTES) {
      setError("Logo image must be smaller than 1.5 MB.");
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") {
        onChange(reader.result);
      }
    };
    reader.onerror = () => {
      setError("Failed to read the selected image.");
    };
    reader.readAsDataURL(file);
  };

  return (
    <div className="space-y-1.5">
      <label className="block text-xs font-medium text-[var(--ink-secondary)]">
        Event Logo (optional)
      </label>

      <div className="flex items-center gap-3 rounded-[10px] border border-[var(--card-line)] bg-[var(--card-solid-subtle)] p-2.5">
        <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-[8px] border border-[var(--card-line)] bg-[var(--card-solid)]">
          {value ? (
            <img
              src={value}
              alt="Event logo preview"
              className="h-full w-full object-cover"
            />
          ) : (
            <ImagePlus className="h-4 w-4 text-[var(--ink-tertiary)]" />
          )}
        </div>

        <div className="flex flex-1 flex-wrap items-center gap-2 min-w-0">
          <input
            ref={inputRef}
            type="file"
            accept="image/*"
            onChange={handleFileChange}
            disabled={disabled}
            className="hidden"
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onClick={() => inputRef.current?.click()}
            className="h-8 text-xs gap-1.5"
          >
            <Upload className="h-3.5 w-3.5" />
            <span>{value ? "Change logo" : "Upload logo"}</span>
          </Button>

          {value && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={disabled}
              onClick={() => {
                setError(null);
                onChange("");
              }}
              className="h-8 px-2 text-xs text-[var(--color-coral)] hover:bg-[var(--color-coral-dim)] gap-1"
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span>Remove</span>
            </Button>
          )}
          <span className="text-[11px] text-[var(--ink-tertiary)]">
            PNG, JPG, WebP, or SVG up to 1.5 MB
          </span>
        </div>
      </div>

      {error && (
        <p className="text-[11px] text-[var(--color-coral)]">{error}</p>
      )}
    </div>
  );
}
