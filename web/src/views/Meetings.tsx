import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { Play, Plus, Send } from "lucide-react";
import { ago, post, type Agent, type Board, type Meeting, type Role } from "../lib";
import {
  qk, useAgents, useBoards, useMeeting, useMeetingMessages, useMeetings, useRoles,
} from "../queries";
import {
  Badge, Button, Empty, Field, Input, Modal, Panel, Select, Spinner, Textarea, stateTone,
} from "../ui";

export default function MeetingsView() {
  const { data: meetings } = useMeetings();
  const { data: agents } = useAgents();
  const { data: roles } = useRoles();
  const { data: boards } = useBoards();
  const [open, setOpen] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  return (
    <div className="grid gap-4 lg:grid-cols-[22rem_1fr]">
      <Panel
        title="Rooms"
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus className="size-3.5" /> call a meeting
          </Button>
        }
      >
        {(meetings?.length ?? 0) === 0 && <Empty>No meetings yet.</Empty>}
        <ul className="divide-y divide-ink-800">
          {meetings?.map((m) => (
            <li key={m.id}>
              <button
                onClick={() => setOpen(m.id)}
                className="w-full py-2.5 text-left transition-opacity hover:opacity-80"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-[13px]">{m.title}</span>
                  <Badge tone={stateTone(m.state)}>{m.state}</Badge>
                </div>
                <div className="mt-0.5 text-[11px] text-ink-500">
                  {m.kind} · round {m.round}/{m.max_rounds} · {ago(m.created_at)}
                </div>
              </button>
            </li>
          ))}
        </ul>
      </Panel>

      {open ? (
        <Room id={open} agents={agents ?? []} />
      ) : (
        <Panel title="Transcript"><Empty>Pick a room.</Empty></Panel>
      )}

      <AnimatePresence>
        {creating && (
          <NewMeeting
            agents={agents ?? []}
            roles={roles ?? []}
            boards={boards ?? []}
            onClose={() => setCreating(false)}
            onOpened={setOpen}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

function Room({ id, agents }: { id: string; agents: Agent[] }) {
  const qc = useQueryClient();
  const { data: meeting } = useMeeting(id);
  const { data: messages } = useMeetingMessages(id);
  const [draft, setDraft] = useState("");

  const run = useMutation({
    mutationFn: () => post(`/meetings/${id}/run`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.meetings }),
  });

  const say = useMutation({
    mutationFn: (content: string) => post(`/meetings/${id}/messages`, { content }),
    onSuccess: () => {
      setDraft("");
      qc.invalidateQueries({ queryKey: qk.meetingMessages(id) });
    },
  });

  if (!meeting) return <Panel title="Transcript"><Empty>loading…</Empty></Panel>;

  return (
    <Panel
      title={meeting.title}
      actions={
        <>
          <Badge tone={stateTone(meeting.state)}>{meeting.state}</Badge>
          <Button
            variant="primary"
            onClick={() => run.mutate()}
            disabled={run.isPending || !!meeting.closed_at}
          >
            {run.isPending ? <Spinner /> : <Play className="size-3.5" />} run discussion
          </Button>
        </>
      }
    >
      <div className="mb-3 rounded-md border border-ink-800 bg-ink-950 p-3">
        <div className="text-[11px] tracking-wide text-ink-500 uppercase">Agenda</div>
        <pre className="mt-1 font-mono text-[12px] whitespace-pre-wrap text-ink-300">
          {meeting.agenda}
        </pre>
        <div className="mt-2 flex flex-wrap gap-1">
          {meeting.participants.map((p) => (
            <Badge key={p}>{agents.find((a) => a.id === p)?.name ?? p.slice(0, 8)}</Badge>
          ))}
        </div>
      </div>

      <div className="max-h-[28rem] space-y-3 overflow-y-auto pr-1">
        {(messages?.length ?? 0) === 0 && <Empty>Nothing said yet.</Empty>}
        <AnimatePresence initial={false}>
          {messages?.map((m) => (
            <motion.article
              key={m.id}
              layout
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              className="rounded-md border border-ink-800 bg-ink-850 p-3"
            >
              <div className="flex items-center gap-2 text-[11px] text-ink-500">
                <span className="font-medium text-ink-300">{m.speaker}</span>
                <span>round {m.round}</span>
                <span>{ago(m.created_at)}</span>
              </div>
              <p className="mt-1.5 text-[13px] leading-relaxed whitespace-pre-wrap">{m.content}</p>
            </motion.article>
          ))}
        </AnimatePresence>
      </div>

      {meeting.decision && (
        <motion.div
          initial={{ opacity: 0, scale: 0.98 }}
          animate={{ opacity: 1, scale: 1 }}
          className="mt-3 rounded-md border border-signal/30 bg-signal/5 p-3"
        >
          <div className="text-[11px] tracking-wide text-signal uppercase">Decision</div>
          <p className="mt-1 text-[13px] whitespace-pre-wrap">{meeting.decision}</p>
        </motion.div>
      )}

      <div className="mt-3 flex gap-2">
        <Input
          placeholder="speak as the Founder…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && draft.trim() && say.mutate(draft)}
        />
        <Button onClick={() => draft.trim() && say.mutate(draft)} disabled={say.isPending}>
          {say.isPending ? <Spinner /> : <Send className="size-3.5" />} send
        </Button>
      </div>
    </Panel>
  );
}

function NewMeeting({
  agents, roles, boards, onClose, onOpened,
}: {
  agents: Agent[]; roles: Role[]; boards: Board[];
  onClose: () => void; onOpened: (id: string) => void;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState({
    title: "", kind: "grooming", agenda: "", max_rounds: 3,
    board_id: boards[0]?.id ?? "",
  });
  const [picked, setPicked] = useState<string[]>([]);

  const save = useMutation({
    mutationFn: () =>
      post<Meeting>("/meetings", { ...form, board_id: form.board_id || null, participants: picked }),
    onSuccess: (m) => {
      qc.invalidateQueries({ queryKey: qk.meetings });
      onOpened(m.id);
      onClose();
    },
  });

  return (
    <Modal title="Call a meeting" onClose={onClose} wide>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Title">
          <Input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
        </Field>
        <Field label="Kind">
          <Select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
            {["grooming", "standup", "design", "retro", "incident"].map((k) => (
              <option key={k} value={k}>{k}</option>
            ))}
          </Select>
        </Field>
      </div>
      <Field label="Agenda" hint="State it as the questions that must be settled.">
        <Textarea rows={6} value={form.agenda} onChange={(e) => setForm({ ...form, agenda: e.target.value })} />
      </Field>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Board" hint="Tasks the meeting produces land here.">
          <Select value={form.board_id} onChange={(e) => setForm({ ...form, board_id: e.target.value })}>
            <option value="">none</option>
            {boards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
          </Select>
        </Field>
        <Field label="Max rounds" hint="The facilitator must decide by then.">
          <Input
            type="number"
            min={1}
            value={form.max_rounds}
            onChange={(e) => setForm({ ...form, max_rounds: Number(e.target.value) })}
          />
        </Field>
      </div>
      <Field label="Participants">
        <div className="flex flex-wrap gap-2">
          {agents.map((a) => {
            const on = picked.includes(a.id);
            return (
              <button
                key={a.id}
                onClick={() => setPicked(on ? picked.filter((p) => p !== a.id) : [...picked, a.id])}
                className={
                  "rounded-md border px-2 py-1 text-[12px] transition-colors " +
                  (on
                    ? "border-signal/40 bg-signal/15 text-signal"
                    : "border-ink-700 bg-ink-850 text-ink-300 hover:border-ink-500")
                }
              >
                {a.name}
                <span className="ml-1 text-ink-500">
                  {roles.find((r) => r.id === a.role_id)?.slug}
                </span>
              </button>
            );
          })}
        </div>
      </Field>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !form.title.trim() || picked.length === 0}
        >
          {save.isPending ? <Spinner /> : null} open room
        </Button>
      </div>
    </Modal>
  );
}
