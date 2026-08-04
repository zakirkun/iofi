import type { ReactNode } from "react";
import { cx } from "./lib";

export function Panel({
  title,
  actions,
  children,
  className,
}: {
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      className={cx(
        "rounded-lg border border-ink-800 bg-ink-900/70 backdrop-blur-sm",
        className,
      )}
    >
      {(title || actions) && (
        <header className="flex items-center justify-between gap-3 border-b border-ink-800 px-4 py-2.5">
          <h2 className="text-[13px] font-medium tracking-wide text-ink-300 uppercase">
            {title}
          </h2>
          <div className="flex items-center gap-2">{actions}</div>
        </header>
      )}
      <div className="p-4">{children}</div>
    </section>
  );
}

export function Button({
  children,
  onClick,
  variant = "default",
  disabled,
  type = "button",
  className,
}: {
  children: ReactNode;
  onClick?: () => void;
  variant?: "default" | "primary" | "ghost" | "danger";
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
}) {
  const styles = {
    default: "bg-ink-800 hover:bg-ink-700 text-ink-100 border-ink-700",
    primary: "bg-signal/15 hover:bg-signal/25 text-signal border-signal/30",
    ghost: "bg-transparent hover:bg-ink-800 text-ink-300 border-transparent",
    danger: "bg-alert/10 hover:bg-alert/20 text-alert border-alert/30",
  }[variant];
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={cx(
        "rounded-md border px-2.5 py-1.5 text-[13px] font-medium transition-colors",
        "disabled:cursor-not-allowed disabled:opacity-40",
        styles,
        className,
      )}
    >
      {children}
    </button>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "good" | "warn" | "bad" | "info";
}) {
  const tones = {
    neutral: "bg-ink-800 text-ink-300 border-ink-700",
    good: "bg-signal/10 text-signal border-signal/25",
    warn: "bg-warn/10 text-warn border-warn/25",
    bad: "bg-alert/10 text-alert border-alert/25",
    info: "bg-sky-400/10 text-sky-300 border-sky-400/25",
  }[tone];
  return (
    <span
      className={cx(
        "inline-flex items-center rounded border px-1.5 py-0.5 text-[11px] font-medium",
        tones,
      )}
    >
      {children}
    </span>
  );
}

export const stateTone = (s: string) =>
  ({
    done: "good", running: "info", queued: "warn", failed: "bad",
    idle: "neutral", busy: "info", error: "bad", paused: "warn",
    open: "info", closed: "neutral", decided: "good",
  })[s] as "neutral" | "good" | "warn" | "bad" | "info" | undefined ?? "neutral";

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block space-y-1">
      <span className="text-[12px] font-medium text-ink-300">{label}</span>
      {children}
      {hint && <span className="block text-[11px] text-ink-500">{hint}</span>}
    </label>
  );
}

const inputBase =
  "w-full rounded-md border border-ink-700 bg-ink-950 px-2.5 py-1.5 text-[13px] " +
  "text-ink-100 placeholder:text-ink-500 outline-none focus:border-signal/50";

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cx(inputBase, props.className)} />;
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      {...props}
      className={cx(inputBase, "resize-y font-mono leading-relaxed", props.className)}
    />
  );
}

export function Select(props: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={cx(inputBase, props.className)} />;
}

export function Modal({
  title,
  onClose,
  children,
  wide,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/70 p-6"
      onClick={onClose}
    >
      <div
        className={cx(
          "mt-10 w-full rounded-xl border border-ink-700 bg-ink-900 shadow-2xl",
          wide ? "max-w-4xl" : "max-w-xl",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="flex items-center justify-between border-b border-ink-800 px-5 py-3">
          <h3 className="text-sm font-semibold">{title}</h3>
          <Button variant="ghost" onClick={onClose}>
            close
          </Button>
        </header>
        <div className="space-y-4 p-5">{children}</div>
      </div>
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-[13px] text-ink-500">{children}</p>;
}

export function Stat({ label, value, sub }: { label: string; value: ReactNode; sub?: string }) {
  return (
    <div className="rounded-lg border border-ink-800 bg-ink-900/70 px-4 py-3">
      <div className="text-[11px] tracking-wide text-ink-500 uppercase">{label}</div>
      <div className="mt-1 font-mono text-2xl text-ink-100">{value}</div>
      {sub && <div className="text-[11px] text-ink-500">{sub}</div>}
    </div>
  );
}
