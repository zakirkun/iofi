import { useId, useState, type ReactNode } from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { AlertTriangle, ArrowDown, ArrowUp, Check, Info, Loader2, UploadCloud, X } from "lucide-react";
import { cx } from "./lib";
import { useToasts } from "./store";

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
  title,
}: {
  children: ReactNode;
  onClick?: () => void;
  variant?: "default" | "primary" | "ghost" | "danger";
  disabled?: boolean;
  type?: "button" | "submit";
  className?: string;
  title?: string;
}) {
  const styles = {
    default: "bg-ink-800 hover:bg-ink-700 text-ink-100 border-ink-700",
    primary: "bg-signal/15 hover:bg-signal/25 text-signal border-signal/30",
    ghost: "bg-transparent hover:bg-ink-800 text-ink-300 border-transparent",
    danger: "bg-alert/10 hover:bg-alert/20 text-alert border-alert/30",
  }[variant];
  return (
    <motion.button
      type={type}
      onClick={onClick}
      disabled={disabled}
      title={title}
      // A press that moves confirms the click landed without waiting for the
      // request behind it to answer.
      whileTap={disabled ? undefined : { scale: 0.97 }}
      transition={{ type: "spring", stiffness: 500, damping: 30 }}
      className={cx(
        "inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[13px] font-medium transition-colors",
        "disabled:cursor-not-allowed disabled:opacity-40",
        styles,
        className,
      )}
    >
      {children}
    </motion.button>
  );
}

export function Badge({
  children,
  tone = "neutral",
  title,
}: {
  children: ReactNode;
  tone?: "neutral" | "good" | "warn" | "bad" | "info";
  title?: string;
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
      title={title}
      className={cx(
        "inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[11px] font-medium",
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
    ready: "good", pending: "warn",
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
  "text-ink-100 placeholder:text-ink-500 outline-none transition-colors focus:border-signal/50";

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

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cx("size-3.5 animate-spin", className)} />;
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
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/70 p-6 backdrop-blur-sm"
      onClick={onClose}
    >
      <motion.div
        initial={{ opacity: 0, y: -12, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: -8, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 400, damping: 32 }}
        className={cx(
          "mt-10 w-full rounded-xl border border-ink-700 bg-ink-900 shadow-2xl",
          wide ? "max-w-4xl" : "max-w-xl",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="flex items-center justify-between border-b border-ink-800 px-5 py-3">
          <h3 className="text-sm font-semibold">{title}</h3>
          <Button variant="ghost" onClick={onClose} title="close">
            <X className="size-4" />
          </Button>
        </header>
        <div className="space-y-4 p-5">{children}</div>
      </motion.div>
    </motion.div>
  );
}

/** A side panel for things read alongside the list they came from — a task next
 *  to its board, a document next to its shelf. A modal would hide the context
 *  that makes the contents mean anything. */
export function Drawer({
  title,
  subtitle,
  onClose,
  children,
  actions,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  onClose: () => void;
  children: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <motion.aside
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 flex justify-end bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <motion.div
        initial={{ x: "100%" }}
        animate={{ x: 0 }}
        exit={{ x: "100%" }}
        transition={{ type: "spring", stiffness: 380, damping: 38 }}
        onClick={(e) => e.stopPropagation()}
        className="flex h-full w-full max-w-3xl flex-col border-l border-ink-700 bg-ink-900 shadow-2xl"
      >
        <header className="flex items-start justify-between gap-3 border-b border-ink-800 px-5 py-3">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold">{title}</h3>
            {subtitle && <div className="mt-0.5 text-[11px] text-ink-500">{subtitle}</div>}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {actions}
            <Button variant="ghost" onClick={onClose} title="close">
              <X className="size-4" />
            </Button>
          </div>
        </header>
        <div className="flex-1 space-y-4 overflow-y-auto p-5">{children}</div>
      </motion.div>
    </motion.aside>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-[13px] text-ink-500">{children}</p>;
}

export function Stat({ label, value, sub }: { label: string; value: ReactNode; sub?: string }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      className="rounded-lg border border-ink-800 bg-ink-900/70 px-4 py-3"
    >
      <div className="text-[11px] tracking-wide text-ink-500 uppercase">{label}</div>
      <div className="mt-1 font-mono text-2xl text-ink-100">{value}</div>
      {sub && <div className="text-[11px] text-ink-500">{sub}</div>}
    </motion.div>
  );
}

/** Sortable, filterable table over any row shape. Every list in the console
 *  that is genuinely tabular uses this, so sorting behaves the same in all of
 *  them and none of them reimplements a header click. */
export function DataTable<T>({
  data,
  columns,
  filter,
  empty = "nothing here",
  initialSort,
}: {
  data: T[];
  columns: ColumnDef<T, any>[];
  filter?: string;
  empty?: ReactNode;
  initialSort?: SortingState;
}) {
  const [sorting, setSorting] = useState<SortingState>(initialSort ?? []);
  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter: filter ?? "" },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  if (data.length === 0) return <Empty>{empty}</Empty>;

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[12px]">
        <thead className="text-ink-500">
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id} className="text-left">
              {hg.headers.map((h) => {
                const sorted = h.column.getIsSorted();
                return (
                  <th key={h.id} className="py-1 font-medium">
                    {h.isPlaceholder ? null : (
                      <button
                        onClick={h.column.getToggleSortingHandler()}
                        disabled={!h.column.getCanSort()}
                        className={cx(
                          "inline-flex items-center gap-1",
                          h.column.getCanSort() && "hover:text-ink-300",
                        )}
                      >
                        {flexRender(h.column.columnDef.header, h.getContext())}
                        {sorted === "asc" && <ArrowUp className="size-3" />}
                        {sorted === "desc" && <ArrowDown className="size-3" />}
                      </button>
                    )}
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody className="font-mono">
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} className="border-t border-ink-800">
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className="py-1">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {table.getRowModel().rows.length === 0 && <Empty>no rows match that filter</Empty>}
    </div>
  );
}

/** File drop target. Built on the native drag events rather than a dependency:
 *  dnd-kit sorts elements within a page and has nothing to do with files coming
 *  in from the desktop. */
export function Dropzone({
  onFiles,
  accept,
  hint,
  busy,
}: {
  onFiles: (files: File[]) => void;
  accept?: string;
  hint?: ReactNode;
  busy?: boolean;
}) {
  const [over, setOver] = useState(false);
  // A hand-built id collides the moment a second dropzone renders, and the
  // label then opens the wrong file picker.
  const inputId = useId();

  return (
    <label
      htmlFor={inputId}
      onDragOver={(e) => {
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        setOver(false);
        const files = Array.from(e.dataTransfer.files);
        if (files.length) onFiles(files);
      }}
      className={cx(
        "flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed px-6 py-10 text-center transition-colors",
        over ? "border-signal/60 bg-signal/5" : "border-ink-700 bg-ink-950/50 hover:border-ink-500",
      )}
    >
      <input
        id={inputId}
        type="file"
        multiple
        accept={accept}
        className="hidden"
        onChange={(e) => {
          const files = Array.from(e.target.files ?? []);
          if (files.length) onFiles(files);
          e.target.value = ""; // re-uploading the same file must fire again
        }}
      />
      {busy ? <Spinner className="size-5 text-signal" /> : <UploadCloud className="size-5 text-ink-500" />}
      <span className="text-[13px] text-ink-300">
        {busy ? "uploading…" : "drop files here, or click to choose"}
      </span>
      {hint && <span className="max-w-md text-[11px] text-ink-500">{hint}</span>}
    </label>
  );
}

const toastIcon = {
  info: <Info className="size-4" />,
  good: <Check className="size-4" />,
  bad: <AlertTriangle className="size-4" />,
};

/** Mounted once at the root. Anything anywhere can call `toast(...)`. */
export function Toaster() {
  const { toasts, dismiss } = useToasts();
  return (
    <div className="pointer-events-none fixed bottom-4 left-1/2 z-[60] flex w-full max-w-md -translate-x-1/2 flex-col gap-2 px-4">
      <AnimatePresence initial={false}>
        {toasts.map((t) => (
          <motion.div
            key={t.id}
            layout
            initial={{ opacity: 0, y: 16, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 8, scale: 0.97 }}
            transition={{ type: "spring", stiffness: 420, damping: 34 }}
            className={cx(
              "pointer-events-auto flex items-start gap-2 rounded-lg border px-3 py-2 text-[12px] shadow-xl backdrop-blur",
              t.tone === "bad" && "border-alert/40 bg-alert/10 text-alert",
              t.tone === "good" && "border-signal/40 bg-signal/10 text-signal",
              t.tone === "info" && "border-ink-700 bg-ink-900/95 text-ink-300",
            )}
          >
            <span className="mt-px shrink-0">{toastIcon[t.tone]}</span>
            <span className="min-w-0 flex-1 break-words whitespace-pre-wrap">{t.text}</span>
            <button onClick={() => dismiss(t.id)} className="shrink-0 opacity-60 hover:opacity-100">
              <X className="size-3.5" />
            </button>
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
