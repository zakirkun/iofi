import { Link, Outlet, useRouterState } from "@tanstack/react-router";
import { motion } from "motion/react";
import {
  Boxes, BrainCircuit, LayoutDashboard, Library, MessagesSquare, Terminal,
} from "lucide-react";
import { cx } from "./lib";
import { useHealth, useLiveSync } from "./queries";
import { Toaster } from "./ui";

export const TABS = [
  { to: "/", label: "Command", icon: LayoutDashboard },
  { to: "/board", label: "Kanban", icon: Boxes },
  { to: "/org", label: "Org", icon: BrainCircuit },
  { to: "/meetings", label: "Meetings", icon: MessagesSquare },
  { to: "/knowledge", label: "Knowledge", icon: Library },
  { to: "/runs", label: "Runs", icon: Terminal },
] as const;

/** The console shell: one SSE subscription for the whole app, the nav, and the
 *  health strip. Views below it read from the query cache and never poll. */
export default function App() {
  useLiveSync(["runs", "boards", "meetings", "memory", "knowledge", "founder"]);

  const path = useRouterState({ select: (s) => s.location.pathname });
  const { data: health } = useHealth();

  const brainDown = health?.litellm && health.litellm !== "up" && health.litellm !== "unknown";
  const doclingDown = health?.docling && health.docling !== "up" && health.docling !== "off";

  return (
    <div className="flex min-h-full flex-col">
      <header className="sticky top-0 z-40 border-b border-ink-800 bg-ink-950/90 backdrop-blur">
        <div className="mx-auto flex max-w-[1600px] flex-wrap items-center gap-x-6 gap-y-2 px-5 py-3">
          <div className="flex items-baseline gap-2">
            <span className="text-lg font-semibold tracking-tight">iofi</span>
            <span className="text-[11px] text-ink-500">you are the Founder</span>
          </div>

          <nav className="flex gap-1">
            {TABS.map((t) => {
              const active = t.to === "/" ? path === "/" : path.startsWith(t.to);
              const Icon = t.icon;
              return (
                <Link
                  key={t.to}
                  to={t.to}
                  className={cx(
                    "relative flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] transition-colors",
                    active ? "text-ink-100" : "text-ink-300 hover:text-ink-100",
                  )}
                >
                  {active && (
                    // One shared layoutId slides the highlight between tabs
                    // instead of it disappearing here and reappearing there.
                    <motion.span
                      layoutId="tab-active"
                      className="absolute inset-0 rounded-md bg-ink-800"
                      transition={{ type: "spring", stiffness: 500, damping: 40 }}
                    />
                  )}
                  <Icon className="relative size-3.5" />
                  <span className="relative">{t.label}</span>
                </Link>
              );
            })}
          </nav>

          <div className="ml-auto flex items-center gap-3 text-[11px] text-ink-500">
            {brainDown && (
              <span className="flex items-center gap-1.5" title={health?.litellm}>
                <span className="size-2 rounded-full bg-warn" />
                coordinator offline — triage and meetings will fail
              </span>
            )}
            {doclingDown && (
              <span className="flex items-center gap-1.5" title={health?.docling}>
                <span className="size-2 rounded-full bg-warn" />
                docling offline — only text uploads will convert
              </span>
            )}
            <span className="flex items-center gap-1.5">
              <span className={cx("size-2 rounded-full", health?.ok ? "bg-signal" : "bg-alert")} />
              {health?.ok ? "database up" : (health?.database ?? "connecting…")}
            </span>
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-[1600px] flex-1 px-5 py-5">
        <Outlet />
      </main>

      <Toaster />
    </div>
  );
}
