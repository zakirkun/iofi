import { useMemo, useState } from "react";
import {
  del, patch, post, put, useFetch, useStream, cx, ago,
  type Agent, type Board, type Repo, type Role, type Task, type TaskEvent,
} from "../lib";
import { Badge, Button, Empty, Field, Input, Modal, Panel, Select, Textarea } from "../ui";

export default function BoardView() {
  const { data: boards, reload: reloadBoards } = useFetch<Board[]>("/boards");
  const [boardId, setBoardId] = useState<string | null>(null);
  const board = boards?.find((b) => b.id === boardId) ?? boards?.[0] ?? null;

  const { data: roles } = useFetch<Role[]>("/roles");
  const { data: agents } = useFetch<Agent[]>("/agents");
  const { data: tasks, reload } = useFetch<Task[]>(
    board ? `/boards/${board.id}/tasks` : null,
  );

  useStream(board ? [`board:${board.id}`] : [], reload);

  const [openTask, setOpenTask] = useState<string | null>(null);
  const [composing, setComposing] = useState(false);
  const [settings, setSettings] = useState(false);
  const [dragging, setDragging] = useState<string | null>(null);
  const [hover, setHover] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const byColumn = useMemo(() => {
    const map = new Map<string, Task[]>();
    for (const col of board?.columns ?? []) map.set(col.key, []);
    for (const t of tasks ?? []) {
      if (!map.has(t.column_key)) map.set(t.column_key, []);
      map.get(t.column_key)!.push(t);
    }
    return map;
  }, [board, tasks]);

  if (!boards) return <Empty>loading…</Empty>;
  if (!board) {
    return (
      <Panel title="No board">
        <Empty>Connect a repository under Command — a board is created with it.</Empty>
      </Panel>
    );
  }

  const move = async (taskId: string, column: string) => {
    setError(null);
    try {
      await patch(`/tasks/${taskId}`, { ColumnKey: column });
      reload();
    } catch (e) {
      // A WIP limit refusal comes back as 409; show it rather than snapping back silently.
      setError(String((e as Error).message));
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <Select value={board.id} onChange={(e) => setBoardId(e.target.value)} className="max-w-52">
          {boards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
        </Select>
        <Button variant="primary" onClick={() => setComposing(true)}>+ brief the company</Button>
        <Button onClick={() => setSettings(true)}>board settings</Button>
        <span className="text-[12px] text-ink-500">{tasks?.length ?? 0} open tasks</span>
        {!board.repo_id && (
          <Badge tone="warn">no repository — agents get a scratch directory, not a worktree</Badge>
        )}
      </div>

      {error && (
        <p className="rounded-md border border-alert/30 bg-alert/5 px-3 py-2 text-[12px] text-alert">
          {error}
        </p>
      )}

      <div className="grid auto-cols-[minmax(15rem,1fr)] grid-flow-col gap-3 overflow-x-auto pb-3">
        {board.columns.map((col) => {
          const list = byColumn.get(col.key) ?? [];
          const over = col.wip > 0 && list.length > col.wip;
          const full = col.wip > 0 && list.length >= col.wip;
          return (
            <div
              key={col.key}
              onDragOver={(e) => {
                e.preventDefault();
                setHover(col.key);
              }}
              onDragLeave={() => setHover((h) => (h === col.key ? null : h))}
              onDrop={(e) => {
                e.preventDefault();
                setHover(null);
                if (dragging) move(dragging, col.key);
                setDragging(null);
              }}
              className={cx(
                "min-w-60 rounded-lg border bg-ink-900/40 transition-colors",
                hover === col.key ? "border-signal/50 bg-signal/5" : "border-ink-800",
              )}
            >
              <div className="flex items-center justify-between border-b border-ink-800 px-3 py-2">
                <span className="text-[12px] font-medium tracking-wide text-ink-300 uppercase">
                  {col.name}
                </span>
                <span
                  className={cx(
                    "font-mono text-[11px]",
                    over ? "text-alert" : full ? "text-warn" : "text-ink-500",
                  )}
                >
                  {list.length}{col.wip > 0 && `/${col.wip}`}
                </span>
              </div>
              <div className="space-y-2 p-2">
                {list.map((t) => (
                  <TaskCard
                    key={t.id}
                    task={t}
                    roles={roles ?? []}
                    agents={agents ?? []}
                    dragging={dragging === t.id}
                    onDragStart={() => setDragging(t.id)}
                    onDragEnd={() => setDragging(null)}
                    onOpen={() => setOpenTask(t.id)}
                  />
                ))}
                {list.length === 0 && (
                  <p className="px-1 py-3 text-center text-[11px] text-ink-500">drop here</p>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {composing && (
        <IntakeModal
          boardId={board.id}
          onClose={() => setComposing(false)}
          onDone={() => { setComposing(false); reload(); }}
        />
      )}
      {settings && (
        <BoardSettings
          board={board}
          onClose={() => setSettings(false)}
          onSaved={() => { setSettings(false); reloadBoards(); }}
        />
      )}
      {openTask && (
        <TaskModal
          id={openTask}
          tasks={tasks ?? []}
          roles={roles ?? []}
          agents={agents ?? []}
          onClose={() => setOpenTask(null)}
          onChanged={reload}
        />
      )}
    </div>
  );
}

function TaskCard({
  task, roles, agents, dragging, onDragStart, onDragEnd, onOpen,
}: {
  task: Task; roles: Role[]; agents: Agent[]; dragging: boolean;
  onDragStart: () => void; onDragEnd: () => void; onOpen: () => void;
}) {
  const role = roles.find((r) => r.id === task.role_required);
  const agent = agents.find((a) => a.id === task.assignee_id);
  return (
    <article
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={onOpen}
      className={cx(
        "cursor-grab rounded-md border border-ink-800 bg-ink-850 p-2.5 hover:border-ink-700 active:cursor-grabbing",
        dragging && "opacity-40",
      )}
    >
      <div className="flex items-start gap-2">
        <span className="mt-0.5 font-mono text-[11px] text-ink-500">#{task.seq}</span>
        <span className="text-[13px] leading-snug">{task.title}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        {role && <Badge tone="info">{role.slug}</Badge>}
        {agent && <Badge>{agent.name}</Badge>}
        {task.priority <= 2 && <Badge tone="bad">P{task.priority}</Badge>}
        {task.depends_on?.length > 0 && <Badge tone="warn">{task.depends_on.length} dep</Badge>}
        {task.labels?.map((l) => <Badge key={l}>{l}</Badge>)}
      </div>
      <div className="mt-2 text-[11px] text-ink-500">{ago(task.updated_at)}</div>
    </article>
  );
}

function BoardSettings({
  board, onClose, onSaved,
}: { board: Board; onClose: () => void; onSaved: () => void }) {
  const { data: repos } = useFetch<Repo[]>("/repos");
  const [name, setName] = useState(board.name);
  const [repoId, setRepoId] = useState(board.repo_id ?? "");
  const [columns, setColumns] = useState(board.columns);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    try {
      await put(`/boards/${board.id}`, {
        ...board, name, repo_id: repoId || null, columns,
      });
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

  const patchCol = (i: number, next: Partial<(typeof columns)[number]>) =>
    setColumns(columns.map((c, j) => (i === j ? { ...c, ...next } : c)));

  return (
    <Modal title="Board settings" onClose={onClose} wide>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name">
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field
          label="Repository"
          hint="Without one, agents work in a scratch directory and cannot open a PR."
        >
          <Select value={repoId} onChange={(e) => setRepoId(e.target.value)}>
            <option value="">none</option>
            {repos?.map((r) => <option key={r.id} value={r.id}>{r.slug}</option>)}
          </Select>
        </Field>
      </div>

      <Field label="Columns" hint="WIP 0 means no limit. A move into a full column is refused.">
        <div className="space-y-2">
          {columns.map((c, i) => (
            <div key={i} className="flex items-center gap-2">
              <Input
                value={c.key}
                onChange={(e) => patchCol(i, { key: e.target.value })}
                className="max-w-40 font-mono"
                placeholder="key"
              />
              <Input
                value={c.name}
                onChange={(e) => patchCol(i, { name: e.target.value })}
                placeholder="Display name"
              />
              <Input
                type="number"
                min={0}
                value={c.wip}
                onChange={(e) => patchCol(i, { wip: Number(e.target.value) })}
                className="max-w-20"
              />
              <Button
                variant="ghost"
                onClick={() => setColumns(columns.filter((_, j) => j !== i))}
              >
                remove
              </Button>
            </div>
          ))}
          <Button onClick={() => setColumns([...columns, { key: "", name: "", wip: 0 }])}>
            + column
          </Button>
        </div>
      </Field>

      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-between">
        <Button
          variant="danger"
          onClick={async () => { await del(`/boards/${board.id}`); onSaved(); }}
        >
          delete board
        </Button>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={onClose}>cancel</Button>
          <Button variant="primary" onClick={save}>save</Button>
        </div>
      </div>
    </Modal>
  );
}

/** The founder states intent; the coordinator picks the role and splits it. */
function IntakeModal({
  boardId, onClose, onDone,
}: { boardId: string; onClose: () => void; onDone: () => void }) {
  const [title, setTitle] = useState("");
  const [spec, setSpec] = useState("");
  const [dispatch, setDispatch] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await post("/tasks/intake", { board_id: boardId, title, spec, dispatch });
      onDone();
    } catch (e) {
      setError(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="Brief the company" onClose={onClose} wide>
      <Field label="What do you want" hint="A sentence. The coordinator picks the role.">
        <Input value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
      </Field>
      <Field label="Detail" hint="Constraints, context, anything the agents cannot infer.">
        <Textarea rows={8} value={spec} onChange={(e) => setSpec(e.target.value)} />
      </Field>
      <label className="flex items-center gap-2 text-[13px] text-ink-300">
        <input type="checkbox" checked={dispatch} onChange={(e) => setDispatch(e.target.checked)} />
        start the agent immediately if it needs only one role
      </label>
      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={submit} disabled={busy || !title.trim()}>
          {busy ? "triaging…" : "send to the board"}
        </Button>
      </div>
    </Modal>
  );
}

function TaskModal({
  id, tasks, roles, agents, onClose, onChanged,
}: {
  id: string; tasks: Task[]; roles: Role[]; agents: Agent[];
  onClose: () => void; onChanged: () => void;
}) {
  const { data, reload } = useFetch<{ task: Task; events: TaskEvent[] }>(`/tasks/${id}`);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [labels, setLabels] = useState<string | null>(null);
  if (!data) return null;
  const { task, events } = data;

  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      reload();
      onChanged();
    } catch (e) {
      setError(String((e as Error).message));
    } finally {
      setBusy(false);
    }
  };

  const toggleDep = (depId: string) => {
    const next = task.depends_on?.includes(depId)
      ? task.depends_on.filter((d) => d !== depId)
      : [...(task.depends_on ?? []), depId];
    act(() => patch(`/tasks/${task.id}`, { DependsOn: next }));
  };

  return (
    <Modal title={`#${task.seq} ${task.title}`} onClose={onClose} wide>
      <div className="grid gap-4 md:grid-cols-3">
        <div className="space-y-3 md:col-span-2">
          <Panel title="Spec">
            <pre className="font-mono text-[12px] whitespace-pre-wrap text-ink-300">
              {task.spec || "—"}
            </pre>
          </Panel>
          <Panel title="Acceptance">
            <pre className="font-mono text-[12px] whitespace-pre-wrap text-ink-300">
              {task.acceptance || "—"}
            </pre>
          </Panel>
          <Panel title="Depends on" >
            <p className="mb-2 text-[11px] text-ink-500">
              Dispatch is refused while any of these is still open.
            </p>
            <div className="max-h-40 space-y-1 overflow-y-auto">
              {tasks.filter((t) => t.id !== task.id).map((t) => (
                <label key={t.id} className="flex items-center gap-2 text-[12px]">
                  <input
                    type="checkbox"
                    checked={task.depends_on?.includes(t.id) ?? false}
                    onChange={() => toggleDep(t.id)}
                  />
                  <span className="font-mono text-ink-500">#{t.seq}</span>
                  <span className="truncate">{t.title}</span>
                </label>
              ))}
            </div>
          </Panel>
          <Panel title="Activity">
            {events.length === 0 && <Empty>nothing yet</Empty>}
            <ul className="space-y-2">
              {events.map((e) => (
                <li key={e.id} className="border-l-2 border-ink-700 pl-3">
                  <div className="flex items-center gap-2 text-[11px] text-ink-500">
                    <Badge>{e.kind}</Badge>
                    <span>{e.actor}</span>
                    <span>{ago(e.created_at)}</span>
                  </div>
                  {Object.keys(e.data ?? {}).length > 0 && (
                    <pre className="mt-1 font-mono text-[11px] whitespace-pre-wrap text-ink-300">
                      {JSON.stringify(e.data, null, 2)}
                    </pre>
                  )}
                </li>
              ))}
            </ul>
          </Panel>
        </div>

        <div className="space-y-3">
          <Field label="Required role">
            <Select
              value={task.role_required ?? ""}
              onChange={(e) => act(() => patch(`/tasks/${task.id}`, { RoleRequired: e.target.value }))}
            >
              <option value="">unassigned</option>
              {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
            </Select>
          </Field>
          <Field label="Assignee">
            <Select
              value={task.assignee_id ?? ""}
              onChange={(e) => act(() => patch(`/tasks/${task.id}`, { AssigneeID: e.target.value }))}
            >
              <option value="">pick automatically</option>
              {agents.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
            </Select>
          </Field>
          <Field label="Priority" hint="1 highest, 5 lowest">
            <Select
              value={String(task.priority)}
              onChange={(e) => act(() => patch(`/tasks/${task.id}`, { Priority: Number(e.target.value) }))}
            >
              {[1, 2, 3, 4, 5].map((p) => <option key={p} value={p}>P{p}</option>)}
            </Select>
          </Field>
          <Field label="Labels" hint="Comma separated. Enter to save.">
            <Input
              value={labels ?? (task.labels ?? []).join(", ")}
              onChange={(e) => setLabels(e.target.value)}
              onKeyDown={(e) => {
                if (e.key !== "Enter") return;
                const next = (labels ?? "").split(",").map((s) => s.trim()).filter(Boolean);
                act(() => patch(`/tasks/${task.id}`, { Labels: next })).then(() => setLabels(null));
              }}
            />
          </Field>

          <div className="space-y-2">
            <Button
              variant="primary"
              disabled={busy}
              className="w-full"
              onClick={() => act(() => post(`/tasks/${task.id}/dispatch`, {}))}
            >
              dispatch agent
            </Button>
            <Button
              className="w-full"
              disabled={busy}
              onClick={() => act(() => post(`/tasks/${task.id}/push`, {}))}
            >
              push to GitHub
            </Button>
            <Button
              className="w-full"
              disabled={busy}
              onClick={() => act(() => patch(`/tasks/${task.id}`, { Close: !task.closed_at }))}
            >
              {task.closed_at ? "reopen" : "close task"}
            </Button>
            <Button
              variant="danger"
              className="w-full"
              disabled={busy}
              onClick={() => act(async () => { await del(`/tasks/${task.id}`); onClose(); })}
            >
              delete task
            </Button>
          </div>
          {error && <p className="text-[12px] text-alert">{error}</p>}
          <p className="font-mono text-[11px] text-ink-500">{task.id}</p>
        </div>
      </div>
    </Modal>
  );
}
