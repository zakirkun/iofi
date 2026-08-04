import { useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext, DragOverlay, PointerSensor, KeyboardSensor, closestCorners,
  useDroppable, useSensor, useSensors,
  type DragEndEvent, type DragStartEvent,
} from "@dnd-kit/core";
import {
  SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { AnimatePresence, motion } from "motion/react";
import { Filter, Plus, Settings2, X } from "lucide-react";
import {
  del, patch, post, put, ago, cx,
  type Agent, type Board, type Role, type Task,
} from "../lib";
import {
  qk, useAgents, useBoardTasks, useBoards, useRepos, useRoles, useTask,
} from "../queries";
import { filtersActive, toast, useBoardUI } from "../store";
import {
  Badge, Button, Drawer, Empty, Field, Input, Modal, Panel, Select, Spinner, Textarea,
} from "../ui";

/** Gap between adjacent cards. A drop between two neighbours takes the midpoint
 *  of their positions, so ordering never needs every row rewritten. */
const POSITION_GAP = 1000;

export default function BoardView() {
  const { data: boards } = useBoards();
  const { boardId, setBoard, openTaskID, openTask, filters, setFilter, clearFilters } = useBoardUI();

  const board = boards?.find((b) => b.id === boardId) ?? boards?.[0] ?? null;
  const { data: roles } = useRoles();
  const { data: agents } = useAgents();
  const { data: tasks, isLoading } = useBoardTasks(board?.id ?? null);

  const [composing, setComposing] = useState(false);
  const [settings, setSettings] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [dragging, setDragging] = useState<Task | null>(null);

  const qc = useQueryClient();
  const key = qk.boardTasks(board?.id ?? "");

  const move = useMutation({
    mutationFn: (v: { id: string; column: string; position: number }) =>
      patch(`/tasks/${v.id}`, { ColumnKey: v.column, Position: v.position }),
    // Without an optimistic write the card springs back to where it was and
    // then jumps to its new home when the refetch lands, which reads as a
    // rejected drop.
    onMutate: async (v) => {
      await qc.cancelQueries({ queryKey: key });
      const before = qc.getQueryData<Task[]>(key);
      qc.setQueryData<Task[]>(key, (old) =>
        (old ?? []).map((t) =>
          t.id === v.id ? { ...t, column_key: v.column, position: v.position } : t,
        ),
      );
      return { before };
    },
    onError: (err, _v, ctx) => {
      // A WIP limit refusal comes back as an error; put the card back where it
      // was and say why, rather than leaving the board showing a move that the
      // server rejected.
      if (ctx?.before) qc.setQueryData(key, ctx.before);
      toast("bad", err instanceof Error ? err.message : String(err));
    },
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });

  const sensors = useSensors(
    // A few pixels of travel before a drag starts, so opening a card by
    // clicking it still works.
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const visible = useMemo(() => {
    const q = filters.query.trim().toLowerCase();
    return (tasks ?? []).filter((t) => {
      if (q && !`#${t.seq} ${t.title} ${t.spec}`.toLowerCase().includes(q)) return false;
      if (filters.roleId && t.role_required !== filters.roleId) return false;
      if (filters.label && !(t.labels ?? []).includes(filters.label)) return false;
      if (filters.unassignedOnly && t.assignee_id) return false;
      return true;
    });
  }, [tasks, filters]);

  const byColumn = useMemo(() => {
    const map = new Map<string, Task[]>();
    for (const col of board?.columns ?? []) map.set(col.key, []);
    for (const t of visible) {
      if (!map.has(t.column_key)) map.set(t.column_key, []);
      map.get(t.column_key)!.push(t);
    }
    for (const list of map.values()) list.sort((a, b) => a.position - b.position);
    return map;
  }, [board, visible]);

  const labels = useMemo(
    () => [...new Set((tasks ?? []).flatMap((t) => t.labels ?? []))].sort(),
    [tasks],
  );

  if (!boards) return <Empty>loading…</Empty>;
  if (!board) {
    return (
      <Panel title="No board">
        <Empty>Connect a repository under Command — a board is created with it.</Empty>
      </Panel>
    );
  }

  const onDragStart = (e: DragStartEvent) =>
    setDragging((tasks ?? []).find((t) => t.id === e.active.id) ?? null);

  const onDragEnd = (e: DragEndEvent) => {
    setDragging(null);
    const { active, over } = e;
    if (!over) return;

    const task = (tasks ?? []).find((t) => t.id === active.id);
    if (!task) return;

    // `over` is either a column (dropped on empty space) or another card.
    const overTask = (tasks ?? []).find((t) => t.id === over.id);
    const column = overTask ? overTask.column_key : String(over.id);
    if (!board.columns.some((c) => c.key === column)) return;

    const list = (byColumn.get(column) ?? []).filter((t) => t.id !== task.id);
    const index = overTask ? list.findIndex((t) => t.id === overTask.id) : list.length;
    const position = positionFor(list, index < 0 ? list.length : index);

    if (task.column_key === column && task.position === position) return;
    move.mutate({ id: task.id, column, position });
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={board.id}
          onChange={(e) => setBoard(e.target.value)}
          className="max-w-52"
        >
          {boards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
        </Select>
        <Button variant="primary" onClick={() => setComposing(true)}>
          <Plus className="size-3.5" /> brief the company
        </Button>
        <Button onClick={() => setShowFilters((v) => !v)}>
          <Filter className="size-3.5" /> filter
          {filtersActive(filters) && <span className="size-1.5 rounded-full bg-signal" />}
        </Button>
        <Button onClick={() => setSettings(true)}>
          <Settings2 className="size-3.5" /> settings
        </Button>
        <span className="text-[12px] text-ink-500">
          {visible.length}
          {visible.length !== (tasks?.length ?? 0) && ` of ${tasks?.length ?? 0}`} open tasks
          {isLoading && <Spinner className="ml-2 inline" />}
        </span>
        {!board.repo_id && (
          <Badge tone="warn">no repository — agents get a scratch directory, not a worktree</Badge>
        )}
      </div>

      <AnimatePresence initial={false}>
        {showFilters && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <div className="flex flex-wrap items-end gap-3 rounded-lg border border-ink-800 bg-ink-900/60 p-3">
              <Field label="Text">
                <Input
                  value={filters.query}
                  placeholder="title, spec or #number"
                  onChange={(e) => setFilter("query", e.target.value)}
                  className="w-64"
                />
              </Field>
              <Field label="Role">
                <Select value={filters.roleId} onChange={(e) => setFilter("roleId", e.target.value)}>
                  <option value="">any</option>
                  {roles?.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
                </Select>
              </Field>
              <Field label="Label">
                <Select value={filters.label} onChange={(e) => setFilter("label", e.target.value)}>
                  <option value="">any</option>
                  {labels.map((l) => <option key={l} value={l}>{l}</option>)}
                </Select>
              </Field>
              <label className="flex items-center gap-2 pb-1.5 text-[13px] text-ink-300">
                <input
                  type="checkbox"
                  checked={filters.unassignedOnly}
                  onChange={(e) => setFilter("unassignedOnly", e.target.checked)}
                />
                unassigned only
              </label>
              <Button variant="ghost" onClick={clearFilters} className="mb-1">
                <X className="size-3.5" /> clear
              </Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onDragCancel={() => setDragging(null)}
      >
        <div className="grid auto-cols-[minmax(15rem,1fr)] grid-flow-col gap-3 overflow-x-auto pb-3">
          {board.columns.map((col) => (
            <ColumnPane
              key={col.key}
              column={col}
              tasks={byColumn.get(col.key) ?? []}
              roles={roles ?? []}
              agents={agents ?? []}
              onOpen={openTask}
            />
          ))}
        </div>

        <DragOverlay>
          {dragging && (
            <div className="rotate-2 opacity-95">
              <TaskCardBody task={dragging} roles={roles ?? []} agents={agents ?? []} />
            </div>
          )}
        </DragOverlay>
      </DndContext>

      <AnimatePresence>
        {composing && (
          <IntakeModal boardId={board.id} onClose={() => setComposing(false)} />
        )}
        {settings && <BoardSettings board={board} onClose={() => setSettings(false)} />}
        {openTaskID && (
          <TaskDrawer
            id={openTaskID}
            tasks={tasks ?? []}
            roles={roles ?? []}
            agents={agents ?? []}
            onClose={() => openTask(null)}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

/** positionFor returns the sort key for dropping at `index` in `list`. */
function positionFor(list: Task[], index: number): number {
  if (list.length === 0) return POSITION_GAP;
  if (index <= 0) return list[0].position - POSITION_GAP;
  if (index >= list.length) return list[list.length - 1].position + POSITION_GAP;
  return (list[index - 1].position + list[index].position) / 2;
}

function ColumnPane({
  column, tasks, roles, agents, onOpen,
}: {
  column: { key: string; name: string; wip: number };
  tasks: Task[];
  roles: Role[];
  agents: Agent[];
  onOpen: (id: string) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: column.key });
  const over = column.wip > 0 && tasks.length > column.wip;
  const full = column.wip > 0 && tasks.length >= column.wip;

  return (
    <div
      ref={setNodeRef}
      className={cx(
        "min-w-60 rounded-lg border bg-ink-900/40 transition-colors",
        isOver ? "border-signal/50 bg-signal/5" : "border-ink-800",
      )}
    >
      <div className="flex items-center justify-between border-b border-ink-800 px-3 py-2">
        <span className="text-[12px] font-medium tracking-wide text-ink-300 uppercase">
          {column.name}
        </span>
        <span
          className={cx(
            "font-mono text-[11px]",
            over ? "text-alert" : full ? "text-warn" : "text-ink-500",
          )}
          title={column.wip > 0 ? `WIP limit ${column.wip}` : "no WIP limit"}
        >
          {tasks.length}{column.wip > 0 && `/${column.wip}`}
        </span>
      </div>
      <SortableContext items={tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
        <div className="min-h-16 space-y-2 p-2">
          {tasks.map((t) => (
            <SortableTask
              key={t.id}
              task={t}
              roles={roles}
              agents={agents}
              onOpen={() => onOpen(t.id)}
            />
          ))}
          {tasks.length === 0 && (
            <p className="px-1 py-3 text-center text-[11px] text-ink-500">drop here</p>
          )}
        </div>
      </SortableContext>
    </div>
  );
}

function SortableTask({
  task, roles, agents, onOpen,
}: { task: Task; roles: Role[]; agents: Agent[]; onOpen: () => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
  });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      {...attributes}
      {...listeners}
      onClick={onOpen}
      className={cx("cursor-grab active:cursor-grabbing", isDragging && "opacity-30")}
    >
      <TaskCardBody task={task} roles={roles} agents={agents} />
    </div>
  );
}

function TaskCardBody({ task, roles, agents }: { task: Task; roles: Role[]; agents: Agent[] }) {
  const role = roles.find((r) => r.id === task.role_required);
  const agent = agents.find((a) => a.id === task.assignee_id);
  return (
    <article className="rounded-md border border-ink-800 bg-ink-850 p-2.5 transition-colors hover:border-ink-700">
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

function BoardSettings({ board, onClose }: { board: Board; onClose: () => void }) {
  const { data: repos } = useRepos();
  const qc = useQueryClient();
  const [name, setName] = useState(board.name);
  const [repoId, setRepoId] = useState(board.repo_id ?? "");
  const [columns, setColumns] = useState(board.columns);

  const done = () => {
    qc.invalidateQueries({ queryKey: qk.boards });
    onClose();
  };

  const save = useMutation({
    mutationFn: () => put(`/boards/${board.id}`, { ...board, name, repo_id: repoId || null, columns }),
    onSuccess: done,
  });
  const remove = useMutation({
    mutationFn: () => del(`/boards/${board.id}`),
    onSuccess: done,
  });

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
              <Button variant="ghost" onClick={() => setColumns(columns.filter((_, j) => j !== i))}>
                <X className="size-3.5" />
              </Button>
            </div>
          ))}
          <Button onClick={() => setColumns([...columns, { key: "", name: "", wip: 0 }])}>
            <Plus className="size-3.5" /> column
          </Button>
        </div>
      </Field>

      <div className="flex justify-between">
        <Button variant="danger" onClick={() => remove.mutate()}>delete board</Button>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={onClose}>cancel</Button>
          <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending}>
            {save.isPending ? <Spinner /> : null} save
          </Button>
        </div>
      </div>
    </Modal>
  );
}

/** The founder states intent; the coordinator picks the role and splits it. */
function IntakeModal({ boardId, onClose }: { boardId: string; onClose: () => void }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [spec, setSpec] = useState("");
  const [dispatch, setDispatch] = useState(true);

  const submit = useMutation({
    mutationFn: () => post("/tasks/intake", { board_id: boardId, title, spec, dispatch }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.boardTasks(boardId) });
      toast("good", "briefed; the coordinator is triaging it");
      onClose();
    },
  });

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
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button
          variant="primary"
          onClick={() => submit.mutate()}
          disabled={submit.isPending || !title.trim()}
        >
          {submit.isPending ? <><Spinner /> triaging…</> : "send to the board"}
        </Button>
      </div>
    </Modal>
  );
}

function TaskDrawer({
  id, tasks, roles, agents, onClose,
}: {
  id: string; tasks: Task[]; roles: Role[]; agents: Agent[]; onClose: () => void;
}) {
  const qc = useQueryClient();
  const { data } = useTask(id);
  const [labels, setLabels] = useState<string | null>(null);

  const refresh = () => {
    qc.invalidateQueries({ queryKey: qk.task(id) });
    qc.invalidateQueries({ queryKey: ["boards"] });
  };

  const act = useMutation({
    mutationFn: (fn: () => Promise<unknown>) => fn(),
    onSuccess: refresh,
  });

  if (!data) {
    return (
      <Drawer title="loading…" onClose={onClose}>
        <Empty>fetching the task</Empty>
      </Drawer>
    );
  }
  const { task, events } = data;

  const toggleDep = (depId: string) => {
    const next = task.depends_on?.includes(depId)
      ? task.depends_on.filter((d) => d !== depId)
      : [...(task.depends_on ?? []), depId];
    act.mutate(() => patch(`/tasks/${task.id}`, { DependsOn: next }));
  };

  return (
    <Drawer
      title={`#${task.seq} ${task.title}`}
      subtitle={<span className="font-mono">{task.id}</span>}
      onClose={onClose}
      actions={act.isPending ? <Spinner /> : null}
    >
      <div className="grid gap-4 lg:grid-cols-3">
        <div className="space-y-3 lg:col-span-2">
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
          <Panel title="Depends on">
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
                <motion.li
                  key={e.id}
                  initial={{ opacity: 0, x: -6 }}
                  animate={{ opacity: 1, x: 0 }}
                  className={cx(
                    "border-l-2 pl-3",
                    e.kind === "escalated" ? "border-warn" : "border-ink-700",
                  )}
                >
                  <div className="flex items-center gap-2 text-[11px] text-ink-500">
                    <Badge tone={e.kind === "escalated" ? "warn" : "neutral"}>{e.kind}</Badge>
                    <span>{e.actor}</span>
                    <span>{ago(e.created_at)}</span>
                  </div>
                  {Object.keys(e.data ?? {}).length > 0 && (
                    <pre className="mt-1 font-mono text-[11px] whitespace-pre-wrap text-ink-300">
                      {JSON.stringify(e.data, null, 2)}
                    </pre>
                  )}
                </motion.li>
              ))}
            </ul>
          </Panel>
        </div>

        <div className="space-y-3">
          <Field label="Required role">
            <Select
              value={task.role_required ?? ""}
              onChange={(e) => act.mutate(() => patch(`/tasks/${task.id}`, { RoleRequired: e.target.value }))}
            >
              <option value="">unassigned</option>
              {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
            </Select>
          </Field>
          <Field label="Assignee">
            <Select
              value={task.assignee_id ?? ""}
              onChange={(e) => act.mutate(() => patch(`/tasks/${task.id}`, { AssigneeID: e.target.value }))}
            >
              <option value="">pick automatically</option>
              {agents.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
            </Select>
          </Field>
          <Field label="Priority" hint="1 highest, 5 lowest">
            <Select
              value={String(task.priority)}
              onChange={(e) => act.mutate(() => patch(`/tasks/${task.id}`, { Priority: Number(e.target.value) }))}
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
                act.mutate(() => patch(`/tasks/${task.id}`, { Labels: next }));
                setLabels(null);
              }}
            />
          </Field>

          <div className="space-y-2">
            <Button
              variant="primary"
              disabled={act.isPending}
              className="w-full justify-center"
              onClick={() => act.mutate(() => post(`/tasks/${task.id}/dispatch`, {}))}
            >
              dispatch agent
            </Button>
            <Button
              className="w-full justify-center"
              disabled={act.isPending}
              onClick={() => act.mutate(() => post(`/tasks/${task.id}/push`, {}))}
            >
              push to GitHub
            </Button>
            <Button
              className="w-full justify-center"
              disabled={act.isPending}
              onClick={() => act.mutate(() => patch(`/tasks/${task.id}`, { Close: !task.closed_at }))}
            >
              {task.closed_at ? "reopen" : "close task"}
            </Button>
            <Button
              variant="danger"
              className="w-full justify-center"
              disabled={act.isPending}
              onClick={() =>
                act.mutate(async () => {
                  await del(`/tasks/${task.id}`);
                  onClose();
                })
              }
            >
              delete task
            </Button>
          </div>
        </div>
      </div>
    </Drawer>
  );
}
