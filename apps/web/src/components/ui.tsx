// Shared UI primitives — restrained, dense, handcrafted (Stripe/Linear-inspired).
"use client";

import { forwardRef, type HTMLAttributes, type ReactNode, type ButtonHTMLAttributes } from "react";
import { Loader2 } from "lucide-react";

/* ---- Button ---- */
type Variant = "primary" | "secondary" | "ghost" | "danger";

const variantClass: Record<Variant, string> = {
  primary:
    "bg-accent-500 text-white hover:bg-accent-600 focus-visible:ring-accent-400/40",
  secondary:
    "bg-base-800 text-base-200 hover:bg-base-700 hover:text-base-100 border border-base-700",
  ghost:
    "text-base-300 hover:text-base-100 hover:bg-base-800",
  danger:
    "bg-neg-400/10 text-neg-400 hover:bg-neg-400/20 border border-neg-400/20",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: "sm" | "md";
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", size = "md", loading, className = "", children, disabled, ...rest }, ref) => {
    const sizeClass =
      size === "sm" ? "h-8 px-3 text-[13px]" : "h-9 px-4 text-sm";
    return (
      <button
        ref={ref}
        disabled={disabled || loading}
        className={`inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 disabled:opacity-50 disabled:pointer-events-none ${sizeClass} ${variantClass[variant]} ${className}`}
        {...rest}
      >
        {loading && <Loader2 className="h-4 w-4 animate-spin" />}
        {children}
      </button>
    );
  },
);
Button.displayName = "Button";

/* ---- Badge ---- */
export type BadgeTone = "green" | "blue" | "gray" | "amber" | "red" | "violet";

const badgeTone: Record<BadgeTone, string> = {
  green: "bg-pos-400/10 text-pos-400 border-pos-400/20",
  blue: "bg-accent-500/10 text-accent-600 border-accent-500/20",
  gray: "bg-base-800 text-base-300 border-base-700",
  amber: "bg-warn-400/10 text-warn-400 border-warn-400/20",
  red: "bg-neg-400/10 text-neg-400 border-neg-400/20",
  violet: "bg-violet-500/10 text-violet-700 border-violet-500/20",
};

export function Badge({
  tone = "gray",
  dot = false,
  children,
  className = "",
}: {
  tone?: BadgeTone;
  dot?: boolean;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium leading-4 ${badgeTone[tone]} ${className}`}
    >
      {dot && <span className="h-1.5 w-1.5 rounded-full bg-current" />}
      {children}
    </span>
  );
}

/* ---- Card ---- */
export function Card({
  className = "",
  children,
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={`surface rounded-xl ${className}`}
      {...rest}
    >
      {children}
    </div>
  );
}

export function CardHeader({
  title,
  subtitle,
  icon,
  action,
}: {
  title: string;
  subtitle?: string;
  icon?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between border-b border-base-700 px-5 py-4">
      <div className="flex items-center gap-3">
        {icon && <div className="text-base-400">{icon}</div>}
        <div>
          <h3 className="text-sm font-semibold text-base-100">{title}</h3>
          {subtitle && <p className="mt-0.5 text-xs text-base-400">{subtitle}</p>}
        </div>
      </div>
      {action}
    </div>
  );
}

/* ---- Table ---- */
export function Table({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div className={`overflow-x-auto ${className}`}>
      <table className="w-full border-collapse text-left text-sm">{children}</table>
    </div>
  );
}

export function THead({ children }: { children: ReactNode }) {
  return (
    <thead>
      <tr className="border-b border-base-700 text-[11px] uppercase tracking-wider text-base-400">
        {children}
      </tr>
    </thead>
  );
}

export function Th({ children, className = "" }: { children?: ReactNode; className?: string }) {
  return <th className={`px-5 py-3 font-medium ${className}`}>{children}</th>;
}

export function Td({ children, className = "" }: { children?: ReactNode; className?: string }) {
  return <td className={`px-5 py-3.5 align-middle ${className}`}>{children}</td>;
}

export function TRow({
  children,
  onClick,
  className = "",
}: {
  children: ReactNode;
  onClick?: () => void;
  className?: string;
}) {
  return (
    <tr
      onClick={onClick}
      className={`border-b border-base-200 transition-colors last:border-0 hover:bg-base-800/60 ${onClick ? "cursor-pointer" : ""} ${className}`}
    >
      {children}
    </tr>
  );
}

/* ---- Skeleton ----- */
export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`skeleton rounded-md ${className}`} />;
}

/* ---- EmptyState ---- */
export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon?: ReactNode;
  title: string;
  body?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-16 text-center">
      {icon && <div className="mb-3 text-base-500">{icon}</div>}
      <p className="text-sm font-medium text-base-200">{title}</p>
      {body && <p className="mt-1 max-w-xs text-xs text-base-400">{body}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}