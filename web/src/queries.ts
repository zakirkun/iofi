import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { api, post, useStream, type BusEvent } from "./lib";
import type {
  Agent, Board, CostLine, Dashboard, Health, KnowledgeChunk, KnowledgeDoc,
  Meeting, MeetingMessage, Memory, Repo, Role, Run, RunEvent, Skill, Task,
  TaskEvent, Tool, ToolSpec, Workspace,
} from "./lib";

/** Query keys in one place. A key typed inline at the call site is a cache
 *  entry nothing else can ever invalidate. */
export const qk = {
  health: ["health"] as const,
  dashboard: ["dashboard"] as const,
  roles: ["roles"] as const,
  roleSkills: (id: string) => ["roles", id, "skills"] as const,
  skills: ["skills"] as const,
  tools: ["tools"] as const,
  toolCatalog: ["tools", "catalog"] as const,
  agents: ["agents"] as const,
  boards: ["boards"] as const,
  boardTasks: (id: string) => ["boards", id, "tasks"] as const,
  task: (id: string) => ["tasks", id] as const,
  memories: (limit: number) => ["memories", limit] as const,
  docs: ["knowledge"] as const,
  doc: (id: string) => ["knowledge", id] as const,
  docChunks: (id: string) => ["knowledge", id, "chunks"] as const,
  meetings: ["meetings"] as const,
  meeting: (id: string) => ["meetings", id] as const,
  meetingMessages: (id: string) => ["meetings", id, "messages"] as const,
  runs: (limit: number) => ["runs", limit] as const,
  run: (id: string) => ["runs", id] as const,
  runEvents: (id: string) => ["runs", id, "events"] as const,
  repos: ["repos"] as const,
  workspaces: ["workspaces"] as const,
  costs: (days: number) => ["costs", days] as const,
};

const get =
  <T,>(path: string) =>
  () =>
    api<T>(path);

export const useHealth = () =>
  useQuery({ queryKey: qk.health, queryFn: get<Health>("/health"), refetchInterval: 30_000 });

export const useDashboard = () =>
  useQuery({ queryKey: qk.dashboard, queryFn: get<Dashboard>("/dashboard") });

export const useRoles = () => useQuery({ queryKey: qk.roles, queryFn: get<Role[]>("/roles") });
export const useAgents = () => useQuery({ queryKey: qk.agents, queryFn: get<Agent[]>("/agents") });
export const useSkills = () => useQuery({ queryKey: qk.skills, queryFn: get<Skill[]>("/skills") });
export const useTools = () => useQuery({ queryKey: qk.tools, queryFn: get<Tool[]>("/tools") });
export const useBoards = () => useQuery({ queryKey: qk.boards, queryFn: get<Board[]>("/boards") });
export const useRepos = () => useQuery({ queryKey: qk.repos, queryFn: get<Repo[]>("/repos") });
export const useWorkspaces = () =>
  useQuery({ queryKey: qk.workspaces, queryFn: get<Workspace[]>("/workspaces") });

/** The catalog changes only when the server binary does, so it is fetched once
 *  and kept for the session rather than re-requested per modal open. */
export const useToolCatalog = () =>
  useQuery({
    queryKey: qk.toolCatalog,
    queryFn: get<{ tools: ToolSpec[]; categories: Record<string, ToolSpec[]> }>("/tools/catalog"),
    staleTime: Infinity,
  });

export const useRoleSkills = (roleId: string | null) =>
  useQuery({
    queryKey: qk.roleSkills(roleId ?? ""),
    queryFn: get<Skill[]>(`/roles/${roleId}/skills`),
    enabled: !!roleId,
  });

export const useBoardTasks = (boardId: string | null) =>
  useQuery({
    queryKey: qk.boardTasks(boardId ?? ""),
    queryFn: get<Task[]>(`/boards/${boardId}/tasks`),
    enabled: !!boardId,
  });

export const useTask = (id: string | null) =>
  useQuery({
    queryKey: qk.task(id ?? ""),
    queryFn: get<{ task: Task; events: TaskEvent[] }>(`/tasks/${id}`),
    enabled: !!id,
  });

export const useMemories = (limit: number) =>
  useQuery({ queryKey: qk.memories(limit), queryFn: get<Memory[]>(`/memories?limit=${limit}`) });

export const useDocs = () => useQuery({ queryKey: qk.docs, queryFn: get<KnowledgeDoc[]>("/knowledge") });

export const useDoc = (id: string | null) =>
  useQuery({
    queryKey: qk.doc(id ?? ""),
    queryFn: get<KnowledgeDoc>(`/knowledge/${id}`),
    enabled: !!id,
    // A document being extracted settles on its own; poll until it does rather
    // than leaving the founder looking at a stale "pending".
    refetchInterval: (q) => (q.state.data?.state === "pending" ? 2000 : false),
  });

export const useDocChunks = (id: string | null) =>
  useQuery({
    queryKey: qk.docChunks(id ?? ""),
    queryFn: get<KnowledgeChunk[]>(`/knowledge/${id}/chunks`),
    enabled: !!id,
  });

export const useMeetings = () =>
  useQuery({ queryKey: qk.meetings, queryFn: get<Meeting[]>("/meetings") });

export const useMeeting = (id: string | null) =>
  useQuery({
    queryKey: qk.meeting(id ?? ""),
    queryFn: get<Meeting>(`/meetings/${id}`),
    enabled: !!id,
  });

export const useMeetingMessages = (id: string | null) =>
  useQuery({
    queryKey: qk.meetingMessages(id ?? ""),
    queryFn: get<MeetingMessage[]>(`/meetings/${id}/messages`),
    enabled: !!id,
  });

export const useRuns = (limit: number) =>
  useQuery({ queryKey: qk.runs(limit), queryFn: get<Run[]>(`/runs?limit=${limit}`) });

export const useRun = (id: string | null) =>
  useQuery({ queryKey: qk.run(id ?? ""), queryFn: get<Run>(`/runs/${id}`), enabled: !!id });

export const useRunEvents = (id: string | null) =>
  useQuery({
    queryKey: qk.runEvents(id ?? ""),
    queryFn: get<RunEvent[]>(`/runs/${id}/events`),
    enabled: !!id,
  });

export const useCosts = (days: number) =>
  useQuery({ queryKey: qk.costs(days), queryFn: get<CostLine[]>(`/costs?days=${days}`) });

export const useKnowledgeSearch = () =>
  useMutation({
    mutationFn: (body: { query: string; scopes?: string[]; doc_id?: string; limit?: number }) =>
      post<KnowledgeChunk[]>("/knowledge/search", body),
  });

export const useMemorySearch = () =>
  useMutation({
    mutationFn: (body: { Text: string; Limit: number }) => post<Memory[]>("/memories/search", body),
  });

/** invalidateFor maps a bus event onto the caches it makes stale.
 *
 *  This is the whole reason the app uses a query cache: one SSE connection at
 *  the root refreshes every panel that shows the changed thing, instead of each
 *  view owning a reload callback and refetching things nobody is looking at. */
function invalidateFor(qc: QueryClient, e: BusEvent) {
  const [prefix, id] = e.topic.split(":");
  const drop = (key: readonly unknown[]) => qc.invalidateQueries({ queryKey: key });

  switch (prefix) {
    case "boards":
      drop(["boards"]);
      drop(qk.dashboard);
      break;
    case "board":
      drop(qk.boardTasks(id));
      drop(qk.dashboard);
      break;
    case "runs":
    case "run":
      drop(["runs"]);
      drop(qk.dashboard);
      break;
    case "meetings":
      drop(qk.meetings);
      drop(qk.dashboard);
      break;
    case "meeting":
      drop(qk.meeting(id));
      drop(qk.meetingMessages(id));
      drop(qk.meetings);
      break;
    case "memory":
      drop(["memories"]);
      break;
    case "knowledge":
      drop(["knowledge"]);
      break;
    case "founder":
      // An escalation blocks a task, so the board and the dashboard both move.
      drop(["boards"]);
      drop(qk.dashboard);
      break;
  }
}

/** Subscribe once, at the root. Views read from the cache and never poll. */
export function useLiveSync(topics: string[]) {
  const qc = useQueryClient();
  useStream(topics, (e) => invalidateFor(qc, e));
}
