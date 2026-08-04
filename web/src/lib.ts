import { useEffect, useRef } from "react";

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

/** Multipart upload. Content-Type is deliberately left unset: the browser must
 *  generate it so the multipart boundary matches the body it actually wrote. */
export const upload = <T,>(path: string, form: FormData) =>
  api<T>(path, { method: "POST", body: form, headers: { "Content-Type": "" } });

export type BusEvent = { topic: string; kind: string; data: unknown };

/** Every kind the server publishes. EventSource delivers named events only to
 *  listeners registered for that name, so an unlisted kind is silently dropped
 *  and the UI stops updating for it with nothing to show why. */
const EVENT_KINDS = [
  "task_created", "task_updated", "task_moved", "task_claimed", "task_split",
  "task_handoff", "started", "finished", "text", "tool_call", "tool_result",
  "usage", "error", "done", "message", "state", "decided", "closed", "created",
  "updated", "opened", "issue_imported", "question", "uploaded", "ready",
  "failed", "deleted",
];

/** One SSE connection per topic set; onEvent is kept in a ref so re-renders
 *  never tear the stream down. */
export function useStream(topics: string[], onEvent: (e: BusEvent) => void) {
  const key = topics.join("|");
  const handler = useRef(onEvent);
  handler.current = onEvent;

  useEffect(() => {
    if (!key) return;
    // EventSource cannot set headers, so the token travels as a query param.
    const qs = key
      .split("|")
      .map((t) => `topic=${encodeURIComponent(t)}`)
      .concat(`token=${encodeURIComponent(getToken())}`)
      .join("&");
    const src = new EventSource(`/api/events?${qs}`);
    const emit = (m: MessageEvent) => {
      try {
        handler.current(JSON.parse(m.data));
      } catch {
        // A malformed frame must not kill the stream for every other topic.
      }
    };
    src.onmessage = emit;
    for (const k of EVENT_KINDS) src.addEventListener(k, emit as EventListener);
    return () => src.close();
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

export const bytes = (n: number) => {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
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

/** One MCP tool an agent can be granted. Served from /tools/catalog, which is
 *  generated from the same list the server registers from. */
export type ToolSpec = {
  name: string;
  category: "board" | "knowledge" | "org" | "collab" | "repo" | "ops";
  description: string;
  mutates?: boolean;
};

export type Agent = {
  id: string; name: string; role_id: string; tool_id: string;
  llm_mode: string; model: string; status: string; capacity: number;
};

export type Column = { key: string; name: string; wip: number };

export type Board = { id: string; slug: string; name: string; repo_id: string | null; columns: Column[] };

export type Task = {
  id: string; board_id: string; seq: number; title: string; spec: string;
  column_key: string; position: number; priority: number; acceptance: string;
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
  tags: string[]; created_at: string; expires_at: string | null; score?: number;
};

export type KnowledgeDoc = {
  id: string; scope: string; scope_ref: string; title: string;
  filename: string; mime: string; bytes: number; sha256: string;
  extractor: string; state: "pending" | "ready" | "failed"; error: string;
  content?: string; tags: string[]; uploaded_by: string;
  created_at: string; updated_at: string; chunks: number;
};

export type KnowledgeChunk = {
  id: number; doc_id: string; idx: number; heading: string; content: string;
  score?: number; doc_title?: string; doc_scope?: string;
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

export type Health = { ok: boolean; database: string; litellm: string; docling: string };

export type Dashboard = {
  agents: Agent[]; roles: Role[]; boards: Board[]; tasks: Task[];
  runs: Run[]; meetings: Meeting[];
  totals: { cost_usd: number; tokens: number; open_tasks: number };
};
