"use client";

import React, {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export interface SelectOption {
  label: string;
  value: string;
  disabled?: boolean;
}

export interface SelectProps
  extends Omit<
    React.SelectHTMLAttributes<HTMLSelectElement>,
    "size" | "multiple"
  > {
  label?: string;
  error?: string;
  helperText?: string;
  options?: SelectOption[];
  placeholder?: string;
}

function getTextContent(node: React.ReactNode): string {
  if (node === null || node === undefined || typeof node === "boolean") {
    return "";
  }
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map(getTextContent).join("");
  }
  if (React.isValidElement<{ children?: React.ReactNode }>(node)) {
    return getTextContent(node.props.children);
  }
  return "";
}

function extractOptionsFromChildren(children: React.ReactNode): SelectOption[] {
  const result: SelectOption[] = [];

  React.Children.forEach(children, (child) => {
    if (!React.isValidElement(child)) return;

    if (child.type === React.Fragment) {
      const fragProps = child.props as { children?: React.ReactNode };
      result.push(...extractOptionsFromChildren(fragProps.children));
      return;
    }

    if (child.type === "option") {
      const optProps = child.props as React.OptionHTMLAttributes<HTMLOptionElement>;
      const label = getTextContent(optProps.children);
      const value =
        optProps.value !== undefined ? String(optProps.value) : label;
      result.push({
        label,
        value,
        disabled: Boolean(optProps.disabled),
      });
    }
  });

  return result;
}

export const Select = React.forwardRef<HTMLButtonElement, SelectProps>(
  (
    {
      className,
      label,
      error,
      helperText,
      id,
      name,
      options,
      children,
      value,
      defaultValue,
      onChange,
      disabled,
      placeholder,
      "aria-label": ariaLabel,
    },
    ref
  ) => {
    const generatedId = useId();
    const selectId =
      id ||
      (label
        ? `${label.toLowerCase().replace(/\s+/g, "-")}-${generatedId}`
        : `select-${generatedId}`);
    const listboxId = `${selectId}-listbox`;

    const resolvedOptions = useMemo<SelectOption[]>(() => {
      if (options) return options;
      return extractOptionsFromChildren(children);
    }, [options, children]);

    const isControlled = value !== undefined;
    const [internalValue, setInternalValue] = useState<string>(() => {
      if (defaultValue !== undefined) return String(defaultValue);
      return resolvedOptions[0]?.value ?? "";
    });

    const currentValue = isControlled ? String(value ?? "") : internalValue;
    const selectedOption =
      resolvedOptions.find((opt) => opt.value === currentValue) ?? null;

    const [open, setOpen] = useState(false);
    const [highlightedIndex, setHighlightedIndex] = useState<number>(-1);
    const [menuPos, setMenuPos] = useState<{
      top: number;
      left: number;
      width: number;
      openUp: boolean;
    } | null>(null);

    const triggerRef = useRef<HTMLButtonElement | null>(null);
    const menuRef = useRef<HTMLDivElement | null>(null);

    const setMergedRef = useCallback(
      (node: HTMLButtonElement | null) => {
        triggerRef.current = node;
        if (typeof ref === "function") {
          ref(node);
        } else if (ref) {
          ref.current = node;
        }
      },
      [ref]
    );

    const updatePosition = useCallback(() => {
      const trigger = triggerRef.current;
      if (!trigger) return;
      const rect = trigger.getBoundingClientRect();
      const estimatedMenuHeight = Math.min(resolvedOptions.length * 36 + 12, 248);
      const spaceBelow = window.innerHeight - rect.bottom - 8;
      const spaceAbove = rect.top - 8;
      const openUp = spaceBelow < estimatedMenuHeight && spaceAbove > spaceBelow;

      setMenuPos({
        top: openUp ? rect.top - 6 : rect.bottom + 6,
        left: rect.left,
        width: rect.width,
        openUp,
      });
    }, [resolvedOptions.length]);

    useLayoutEffect(() => {
      if (!open) return;
      updatePosition();
    }, [open, updatePosition]);

    useEffect(() => {
      if (!open) return;

      const handleOutside = (event: MouseEvent) => {
        const target = event.target as Node;
        if (
          triggerRef.current?.contains(target) ||
          menuRef.current?.contains(target)
        ) {
          return;
        }
        setOpen(false);
      };

      const handleKeyDown = (event: KeyboardEvent) => {
        if (event.key === "Escape") {
          setOpen(false);
          triggerRef.current?.focus();
        }
      };

      const handleReposition = () => updatePosition();

      document.addEventListener("mousedown", handleOutside);
      document.addEventListener("keydown", handleKeyDown);
      window.addEventListener("resize", handleReposition);
      window.addEventListener("scroll", handleReposition, true);

      return () => {
        document.removeEventListener("mousedown", handleOutside);
        document.removeEventListener("keydown", handleKeyDown);
        window.removeEventListener("resize", handleReposition);
        window.removeEventListener("scroll", handleReposition, true);
      };
    }, [open, updatePosition]);

    const commitSelection = useCallback(
      (opt: SelectOption) => {
        if (opt.disabled) return;
        if (!isControlled) {
          setInternalValue(opt.value);
        }
        if (onChange) {
          const syntheticEvent = {
            target: { value: opt.value, name: name ?? "" },
            currentTarget: { value: opt.value, name: name ?? "" },
          } as unknown as React.ChangeEvent<HTMLSelectElement>;
          onChange(syntheticEvent);
        }
        setOpen(false);
        triggerRef.current?.focus();
      },
      [isControlled, name, onChange]
    );

    const openMenu = () => {
      if (disabled) return;
      const currentIdx = resolvedOptions.findIndex(
        (opt) => opt.value === currentValue && !opt.disabled
      );
      const firstEnabled = resolvedOptions.findIndex((opt) => !opt.disabled);
      setHighlightedIndex(currentIdx >= 0 ? currentIdx : firstEnabled);
      setOpen(true);
    };

    const moveHighlight = (delta: number) => {
      if (resolvedOptions.length === 0) return;
      let next = highlightedIndex;
      for (let i = 0; i < resolvedOptions.length; i++) {
        next =
          (next + delta + resolvedOptions.length) % resolvedOptions.length;
        if (!resolvedOptions[next].disabled) {
          setHighlightedIndex(next);
          return;
        }
      }
    };

    const handleTriggerKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
      if (disabled) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        if (!open) {
          openMenu();
        } else {
          moveHighlight(1);
        }
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        if (!open) {
          openMenu();
        } else {
          moveHighlight(-1);
        }
      } else if (e.key === "Home" && open) {
        e.preventDefault();
        const first = resolvedOptions.findIndex((opt) => !opt.disabled);
        if (first >= 0) setHighlightedIndex(first);
      } else if (e.key === "End" && open) {
        e.preventDefault();
        for (let i = resolvedOptions.length - 1; i >= 0; i--) {
          if (!resolvedOptions[i].disabled) {
            setHighlightedIndex(i);
            break;
          }
        }
      } else if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        if (!open) {
          openMenu();
        } else if (
          highlightedIndex >= 0 &&
          resolvedOptions[highlightedIndex]
        ) {
          commitSelection(resolvedOptions[highlightedIndex]);
        }
      } else if (e.key === "Tab" && open) {
        setOpen(false);
      }
    };

    const displayLabel =
      selectedOption?.label ?? placeholder ?? resolvedOptions[0]?.label ?? "";

    return (
      <div className="w-full space-y-1.5 text-left font-sans">
        {(label || error) && (
          <div className="flex items-baseline justify-between gap-2">
            {label && (
              <label
                htmlFor={selectId}
                className="block text-xs font-semibold text-[var(--ink-primary)]"
              >
                {label}
              </label>
            )}
            {error && (
              <span
                role="alert"
                className="ml-auto text-right text-[11px] font-medium text-[var(--color-coral)]"
              >
                {error}
              </span>
            )}
          </div>
        )}

        <div className="relative">
          <button
            id={selectId}
            ref={setMergedRef}
            type="button"
            role="combobox"
            aria-expanded={open}
            aria-haspopup="listbox"
            aria-controls={open ? listboxId : undefined}
            aria-label={ariaLabel}
            aria-invalid={Boolean(error) || undefined}
            disabled={disabled}
            onClick={() => (open ? setOpen(false) : openMenu())}
            onKeyDown={handleTriggerKeyDown}
            className={cn(
              "flex h-[42px] w-full items-center justify-between gap-2 rounded-[14px] border border-[var(--card-line)] bg-[var(--card-solid)] px-3.5 text-left text-xs text-[var(--ink-primary)] transition-[border-color,box-shadow,background-color] duration-150 ease-out focus:border-[var(--color-blue)] focus:outline-none focus:ring-3 focus:ring-[var(--color-blue)]/20 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer",
              open &&
                "border-[var(--color-blue)] ring-3 ring-[var(--color-blue)]/20",
              error &&
                "border-[var(--color-coral)] ring-3 ring-[var(--color-coral)]/20 focus:border-[var(--color-coral)] focus:ring-[var(--color-coral)]/25",
              className
            )}
          >
            <span
              className={cn(
                "truncate",
                !selectedOption && placeholder
                  ? "text-[var(--ink-tertiary)]"
                  : "text-[var(--ink-primary)]"
              )}
            >
              {displayLabel}
            </span>
            <ChevronDown
              className={cn(
                "h-3.5 w-3.5 shrink-0 text-[var(--ink-secondary)] transition-transform duration-150",
                open && "rotate-180 text-[var(--ink-primary)]"
              )}
            />
          </button>
        </div>

        {open &&
          menuPos &&
          typeof document !== "undefined" &&
          createPortal(
            <div
              ref={menuRef}
              id={listboxId}
              role="listbox"
              aria-label={ariaLabel || label}
              style={{
                position: "fixed",
                top: menuPos.top,
                left: menuPos.left,
                minWidth: menuPos.width,
                transform: menuPos.openUp ? "translateY(-100%)" : undefined,
              }}
              className="z-[100] max-h-60 overflow-y-auto rounded-[12px] border border-[var(--card-line)] bg-[var(--card-solid)] p-1.5 shadow-[var(--shadow-container)] font-sans text-xs animate-in fade-in-0 zoom-in-95 duration-100"
            >
              {resolvedOptions.map((opt, idx) => {
                const isSelected = opt.value === currentValue;
                const isHighlighted = idx === highlightedIndex;
                return (
                  <button
                    key={`${opt.value}-${idx}`}
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    disabled={opt.disabled}
                    onMouseEnter={() => setHighlightedIndex(idx)}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => commitSelection(opt)}
                    className={cn(
                      "flex w-full items-center justify-between gap-2 rounded-[8px] px-2.5 py-2 text-left text-xs transition-colors cursor-pointer",
                      opt.disabled && "opacity-40 cursor-not-allowed",
                      isSelected
                        ? "bg-[var(--color-blue-dim)] text-[var(--color-blue)] font-semibold"
                        : isHighlighted
                        ? "bg-[var(--card-solid-subtle)] text-[var(--ink-primary)] font-medium"
                        : "text-[var(--ink-primary)] hover:bg-[var(--card-solid-subtle)] font-medium"
                    )}
                  >
                    <span className="truncate">{opt.label}</span>
                    {isSelected && <Check className="h-3.5 w-3.5 shrink-0" />}
                  </button>
                );
              })}
            </div>,
            document.body
          )}

        {helperText && (
          <p className="text-xs text-[var(--ink-secondary)]">{helperText}</p>
        )}
      </div>
    );
  }
);

Select.displayName = "Select";
