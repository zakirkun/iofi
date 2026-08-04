import { useEffect, useState } from "react";
import { cx, useFetch } from "./lib";
import HomeView from "./views/Home";
import BoardView from "./views/Board";
import OrgView from "./views/Org";
import MeetingsView from "./views/Meetings";
import KnowledgeView from "./views/Knowledge";
import RunsView from "./views/Runs";

const TABS = [
  { id: "home", label: "Command", view: HomeView },
  { id: "board", label: "Kanban", view: BoardView },
  { id: "org", label: "Org", view: OrgView },
  { id: "meetings", label: "Meetings", view: MeetingsView },
  { id: "knowledge", label: "Memory", view: KnowledgeView },
  { id: "runs", label: "Runs", view: RunsView },
] as const;

type TabId = (typeof TABS)[number]["id"];

export default function App() {
  // The hash is the whole router: reloads and bookmarks keep working, and
  // there is no dependency to install for six tabs.
  const [tab, setTab] = useState<TabId>(
    () => (TABS.find((t) => t.id === location.hash.slice(1))?.id ?? "home") as TabId,
  );
  useEffect(() => {
    const sync = () =>
      setTab((TABS.find((t) => t.id === location.hash.slice(1))?.id ?? "home") as TabId);
    addEventListener("hashchange", sync);
    return () => removeEventListener("hashchange", sync);
  }, []);

  const { data: health } = useFetch<{ ok: boolean; database: string; litellm: string }>(
    "/health",
  );
  const View = TABS.find((t) => t.id === tab)!.view;
  const brainDown = health?.litellm && health.litellm !== "up" && health.litellm !== "unknown";

  return (
    <div className="flex min-h-full flex-col">
      <header className="sticky top-0 z-40 border-b border-ink-800 bg-ink-950/90 backdrop-blur">
        <div className="mx-auto flex max-w-[1600px] items-center gap-6 px-5 py-3">
          <div className="flex items-baseline gap-2">
            <span className="text-lg font-semibold tracking-tight">iofi</span>
            <span className="text-[11px] text-ink-500">you are the Founder</span>
          </div>

          <nav className="flex gap-1">
            {TABS.map((t) => (
              <a
                key={t.id}
                href={`#${t.id}`}
                className={cx(
                  "rounded-md px-3 py-1.5 text-[13px] transition-colors",
                  tab === t.id
                    ? "bg-ink-800 text-ink-100"
                    : "text-ink-300 hover:bg-ink-850 hover:text-ink-100",
                )}
              >
                {t.label}
              </a>
            ))}
          </nav>

          <div className="ml-auto flex items-center gap-3 text-[11px] text-ink-500">
            {brainDown && (
              <span className="flex items-center gap-1.5" title={health?.litellm}>
                <span className="size-2 rounded-full bg-warn" />
                coordinator offline — triage and meetings will fail
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
        <View />
      </main>
    </div>
  );
}
