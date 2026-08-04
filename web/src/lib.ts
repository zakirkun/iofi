import { useCallback, useEffect, useRef, useState } from "react";

const TOKEN_KEY = "iofi.token";

export const getToken = () => localStorage.getItem(TOKEN_KEY) ?? "";
export const setToken = (t: string) => localStorage.setItem(TOKEN_KEY, t);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

/** Raised on 401 so the shell can send the founder back to the token prompt
 *  instead of rendering a wall of "unauthorized" in every panel. */
export class Unauthorized extends Error {}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getToken()}`,
      ...(init?.headers ?? {}),
    },
  });
  if (res.status === 401) {
    clearToken();
    dispatchEvent(new Event("iofi:unauthorized"));
    throw new Unauthorized("unauthorized");
  }
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body || res.statusText);
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

export const post = <T,>(path: string, body: unknown) =>
  api<T>(path, { method: "POST", body: JSON.stringify(body) });
export const patch = <T,>(path: string, body: unknown) =>
  api<T>(path, { method: "PATCH", body: JSON.stringify(body) });
export const put = <T,>(path: string, body: unknown) =>
  api<T>(path, { method: "PUT", body: JSON.stringify(body) });
export const del = (path: string) => api<void>(path, { method: "DELETE" });

/** Fetch on mount, expose a refetch for after mutations.
 *  The in-flight request is aborted when the path changes or the component
 *  unmounts, so switching tabs quickly cannot land a stale response. */
export function useFetch<T>(path: string | null, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(!!path);
  const inFlight = useRef<AbortController | null>(null);

  const reload = useCallback(() => {
    if (!path) return;
    inFlight.current?.abort();
    const ctrl = new AbortController();
    inFlight.current = ctrl;
    setLoading(true);
    api<T>(path, { signal: ctrl.signal })
      .then((d) => {
        if (ctrl.signal.aborted) return;
        setData(d);
        setError(null);
      })
      .catch((e) => {
        if (ctrl.signal.aborted || e?.name === "AbortError") return;
        setError(String(e.message ?? e));
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, ...deps]);

  useEffect(() => {
    reload();
    return () => inFlight.current?.abort();
  }, [reload]);

  return { data, error, loading, reload };
}

export type BusEvent = { topic: string; kind: string; data: unknown };

/** One SSE connection per topic set; onEvent is kept in a ref so re-renders
 *  never tear the stream down. */
export function useStream(topics: string[], onEvent: (e: BusEvent) => void) {
  const key = topics.join("|");
  const handler = useRef(onEvent);
  handler.current = onEvent;

  useEffect(() => {
    if (!key) return;
    // EventSource cannot set headers, so the token travels as a query param.
    const qs = topics
      .map((t) => `topic=${encodeURIComponent(t)}`)
      .concat(`token=${encodeURIComponent(getToken())}`)
      .join("&");
    const src = new EventSource(`/api/events?${qs}`);
    src.onmessage = (m) => handler.current(JSON.parse(m.data));
    // Named events arrive with their own type; catch them all.
    const kinds = [
      "task_created", "task_updated", "task_moved", "task_claimed", "task_split",
      "task_handoff", "started", "finished", "text", "tool_call", "tool_result",
      "usage", "error", "done", "message", "state", "decided", "closed", "created",
      "opened", "issue_imported",
    ];
    for (const k of kinds) {
      src.addEventListener(k, (m) => handler.current(JSON.parse((m as MessageEvent).data)));
    }
    return () => src.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
}

export const cx = (...parts: (string | false | null | undefined)[]) =>
  parts.filter(Boolean).join(" ");

export const money = (n: number) => `$${n.toFixed(4)}`;

export const shortId = (id?: string | null) => (id ? id.slice(0, 8) : "—");

export const ago = (iso?: string | null) => {
  if (!iso) return "—";
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
};

// ---------------------------------------------------------------- types

export type Role = {
  id: string; slug: string; name: string; description: string;
  system_prompt: string; model_pref: string;
  tool_policy: { allow?: string[]; deny?: string[] };
};

export type Skill = {
  id: string; slug: string; version: number; body: string;
  frontmatter: Record<string, unknown>; source_path: string;
};

export type Tool = {
  id: string; kind: string; display: string; transport: string;
  command: string; args: string[]; env: Record<string, string>;
  parse_mode: string; mcp_style: string; mcp_target: string; enabled: boolean;
};

export type Agent = {
  id: string; name: string; role_id: string; tool_id: string;
  llm_mode: string; model: string; status: string; capacity: number;
};

export type Column = { key: string; name: string; wip: number };

export type Board = { id: string; slug: string; name: string; repo_id: string | null; columns: Column[] };

export type Task = {
  id: string; board_id: string; seq: number; title: string; spec: string;
  column_key: string; priority: number; acceptance: string;
  role_required: string | null; assignee_id: string | null;
  depends_on: string[]; labels: string[];
  created_at: string; updated_at: string; closed_at: string | null;
};

export type TaskEvent = {
  id: number; actor: string; kind: string;
  data: Record<string, unknown>; created_at: string;
};

export type Memory = {
  id: string; scope: string; kind: string; title: string; body: string;
  tags: string[]; created_at: string; score?: number;
};

export type Meeting = {
  id: string; kind: string; title: string; agenda: string; state: string;
  round: number; max_rounds: number; participants: string[];
  decision: string; created_at: string; closed_at: string | null;
};

export type MeetingMessage = {
  id: number; speaker: string; round: number; content: string; created_at: string;
};

export type Run = {
  id: string; task_id: string | null; agent_id: string; state: string;
  tokens_in: number; tokens_out: number; cost_usd: number; error: string;
  created_at: string; finished_at: string | null;
};

export type RunEvent = { id: number; seq: number; kind: string; payload: Record<string, unknown> };

export type Repo = {
  id: string; slug: string; remote_url: string; default_branch: string;
  installed: boolean; webhook_secret?: string;
};

export type Workspace = {
  id: string; agent_id: string; repo_id: string | null; task_id: string | null;
  path: string; branch: string; state: string; created_at: string;
};

export type CostLine = {
  agent_id: string; agent_name: string; role_slug: string;
  runs: number; failed: number; tokens_in: number; tokens_out: number; cost_usd: number;
};

export type TokenPrice = { in: number; out: number };

export type Dashboard = {
  agents: Agent[]; roles: Role[]; boards: Board[]; tasks: Task[];
  runs: Run[]; meetings: Meeting[];
  totals: { cost_usd: number; tokens: number; open_tasks: number };
};
