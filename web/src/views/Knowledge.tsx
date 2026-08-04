import { useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import {
  AlertTriangle, FileText, Globe, Layers, RefreshCw, Search, Trash2, UserSquare,
} from "lucide-react";
import {
  ago, bytes, cx, del, post, upload,
  type KnowledgeChunk, type KnowledgeDoc, type Memory,
} from "../lib";
import {
  qk, useBoards, useDoc, useDocChunks, useDocs, useKnowledgeSearch, useMemories,
  useMemorySearch, useRoles,
} from "../queries";
import { toast, useKnowledgeUI } from "../store";
import {
  Badge, Button, Drawer, Empty, Field, Input, Modal, Panel, Select, Spinner,
  Textarea, Dropzone, stateTone,
} from "../ui";

const MEMORY_KINDS = ["fact", "decision", "convention", "pitfall", "artifact"];
const PAGE = 50;

/** Formats the server accepts. Text ones work with nothing else running;
 *  the rest need the docling service. */
const ACCEPT =
  ".md,.markdown,.txt,.text,.csv,.tsv,.json,.yaml,.yml,.log,.rst,.adoc,.asciidoc," +
  ".pdf,.docx,.pptx,.xlsx,.html,.htm,.png,.jpg,.jpeg,.tiff,.tif,.bmp,.webp";

export default function KnowledgeView() {
  const [tab, setTab] = useState<"docs" | "memory">("docs");

  return (
    <div className="space-y-4">
      <div className="flex gap-1 rounded-lg border border-ink-800 bg-ink-900/60 p-1">
        {(
          [
            ["docs", "Reference documents", Layers],
            ["memory", "Shared memory", FileText],
          ] as const
        ).map(([id, label, Icon]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cx(
              "relative flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-1.5 text-[13px] transition-colors",
              tab === id ? "text-ink-100" : "text-ink-500 hover:text-ink-300",
            )}
          >
            {tab === id && (
              <motion.span
                layoutId="kb-tab"
                className="absolute inset-0 rounded-md bg-ink-800"
                transition={{ type: "spring", stiffness: 500, damping: 40 }}
              />
            )}
            <Icon className="relative size-3.5" />
            <span className="relative">{label}</span>
          </button>
        ))}
      </div>

      {tab === "docs" ? <Documents /> : <SharedMemory />}
    </div>
  );
}

// ---------------------------------------------------------------- documents

function Documents() {
  const qc = useQueryClient();
  const { data: docs, isLoading } = useDocs();
  const { data: roles } = useRoles();
  const { data: boards } = useBoards();
  const { openDocID, openDoc, scope, setScope } = useKnowledgeUI();

  const [target, setTarget] = useState<{ scope: string; ref: string }>({ scope: "global", ref: "" });
  const [tags, setTags] = useState("");
  const [query, setQuery] = useState("");

  const search = useKnowledgeSearch();

  const send = useMutation({
    mutationFn: async (files: File[]) => {
      const form = new FormData();
      for (const f of files) form.append("files", f);
      form.append("scope", target.scope);
      form.append("scope_ref", target.ref);
      if (tags.trim()) form.append("tags", tags.trim());
      return upload<{ accepted: KnowledgeDoc[]; rejected: Record<string, string> }>(
        "/knowledge",
        form,
      );
    },
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: qk.docs });
      const bad = Object.entries(res.rejected ?? {});
      if (res.accepted?.length) {
        toast(
          "good",
          `${res.accepted.length} document(s) accepted — extraction runs in the background`,
        );
      }
      for (const [name, why] of bad) toast("bad", `${name}: ${why}`);
    },
  });

  const scopeName = (d: KnowledgeDoc) => {
    if (d.scope === "role") return roles?.find((r) => r.id === d.scope_ref)?.slug ?? "role";
    if (d.scope === "project") return boards?.find((b) => b.id === d.scope_ref)?.name ?? "project";
    return "everyone";
  };

  const visible = useMemo(
    () => (docs ?? []).filter((d) => scope === "all" || d.scope === scope),
    [docs, scope],
  );

  const failed = (docs ?? []).filter((d) => d.state === "failed").length;

  return (
    <div className="space-y-4">
      <Panel title="Upload reference material">
        <div className="grid gap-3 md:grid-cols-3">
          <Field
            label="Who can read it"
            hint="Global reaches every role. Role-scoped material is invisible to the others."
          >
            <Select
              value={target.scope}
              onChange={(e) => setTarget({ scope: e.target.value, ref: "" })}
            >
              <option value="global">everyone</option>
              <option value="role">one role</option>
              <option value="project">one board</option>
            </Select>
          </Field>
          {target.scope === "role" && (
            <Field label="Role">
              <Select value={target.ref} onChange={(e) => setTarget({ ...target, ref: e.target.value })}>
                <option value="">pick a role…</option>
                {roles?.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </Field>
          )}
          {target.scope === "project" && (
            <Field label="Board">
              <Select value={target.ref} onChange={(e) => setTarget({ ...target, ref: e.target.value })}>
                <option value="">pick a board…</option>
                {boards?.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
              </Select>
            </Field>
          )}
          <Field label="Tags" hint="Comma separated, optional.">
            <Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="contract, legal" />
          </Field>
        </div>

        <div className="mt-3">
          <Dropzone
            accept={ACCEPT}
            busy={send.isPending}
            onFiles={(files) => {
              if (target.scope !== "global" && !target.ref) {
                toast("bad", `pick which ${target.scope === "role" ? "role" : "board"} this is for`);
                return;
              }
              send.mutate(files);
            }}
            hint={
              <>
                Markdown, text, CSV, JSON, YAML and AsciiDoc are read directly. PDF, Word,
                PowerPoint, Excel, HTML and images are converted by docling — if that service
                is not running, those are refused rather than stored unreadable.
              </>
            }
          />
        </div>
      </Panel>

      <Panel
        title={`Documents (${visible.length})`}
        actions={
          <>
            {failed > 0 && (
              <Badge tone="bad">
                <AlertTriangle className="size-3" /> {failed} failed
              </Badge>
            )}
            <Select
              value={scope}
              onChange={(e) => setScope(e.target.value as Parameters<typeof setScope>[0])}
              className="max-w-40"
            >
              <option value="all">all scopes</option>
              <option value="global">global</option>
              <option value="role">role</option>
              <option value="project">project</option>
            </Select>
          </>
        }
      >
        {isLoading && <Empty>loading…</Empty>}
        {!isLoading && visible.length === 0 && (
          <Empty>Nothing uploaded yet. Agents fall back to their own memory alone.</Empty>
        )}
        <ul className="divide-y divide-ink-800">
          <AnimatePresence initial={false}>
            {visible.map((d) => (
              <motion.li
                key={d.id}
                layout
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, height: 0 }}
                className="flex items-center justify-between gap-3 py-2.5"
              >
                <button onClick={() => openDoc(d.id)} className="min-w-0 flex-1 text-left">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[13px] font-medium">{d.title}</span>
                    <Badge tone={stateTone(d.state)}>
                      {d.state === "pending" && <Spinner className="size-2.5" />}
                      {d.state}
                    </Badge>
                    <Badge tone={d.scope === "global" ? "info" : "neutral"}>
                      {d.scope === "global" ? <Globe className="size-3" /> : <UserSquare className="size-3" />}
                      {scopeName(d)}
                    </Badge>
                    {d.tags?.map((t) => <Badge key={t}>{t}</Badge>)}
                  </div>
                  <div className="mt-0.5 flex flex-wrap gap-2 font-mono text-[11px] text-ink-500">
                    <span>{d.filename}</span>
                    <span>{bytes(d.bytes)}</span>
                    <span>{d.chunks} sections</span>
                    {d.extractor && <span>via {d.extractor}</span>}
                    <span>{ago(d.created_at)}</span>
                  </div>
                  {d.state === "failed" && (
                    <p className="mt-1 text-[11px] text-alert">{d.error}</p>
                  )}
                </button>
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
      </Panel>

      <Panel title="Search what the company was given">
        <div className="flex flex-wrap gap-2">
          <Input
            placeholder="ask the documents a question…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && query.trim() && search.mutate({ query, limit: 8 })}
            className="max-w-xl"
          />
          <Button onClick={() => query.trim() && search.mutate({ query, limit: 8 })}>
            {search.isPending ? <Spinner /> : <Search className="size-3.5" />} search
          </Button>
          {search.data && (
            <Button variant="ghost" onClick={() => search.reset()}>clear</Button>
          )}
        </div>
        {search.data && (
          <div className="mt-3 space-y-2">
            {search.data.length === 0 && <Empty>No passage matches. Agents would get the same answer.</Empty>}
            {search.data.map((c) => <ChunkCard key={c.id} chunk={c} onOpen={() => openDoc(c.doc_id)} />)}
          </div>
        )}
      </Panel>

      <AnimatePresence>
        {openDocID && <DocDrawer id={openDocID} onClose={() => openDoc(null)} />}
      </AnimatePresence>
    </div>
  );
}

function ChunkCard({ chunk, onOpen }: { chunk: KnowledgeChunk; onOpen?: () => void }) {
  return (
    <article className="rounded-md border border-ink-800 bg-ink-850 p-3">
      <div className="flex flex-wrap items-center gap-2 text-[11px] text-ink-500">
        <button onClick={onOpen} className="font-medium text-ink-300 hover:text-ink-100">
          {chunk.doc_title ?? "document"}
        </button>
        {chunk.heading && <span>› {chunk.heading}</span>}
        <span className="font-mono">section {chunk.idx}</span>
        {chunk.score !== undefined && chunk.score > 0 && (
          <span className="font-mono">{chunk.score.toFixed(3)}</span>
        )}
      </div>
      <p className="mt-1.5 text-[12px] leading-relaxed whitespace-pre-wrap text-ink-300">
        {chunk.content}
      </p>
    </article>
  );
}

function DocDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const qc = useQueryClient();
  const { data: doc } = useDoc(id);
  const { data: chunks } = useDocChunks(id);

  const refresh = () => {
    qc.invalidateQueries({ queryKey: qk.docs });
    qc.invalidateQueries({ queryKey: qk.doc(id) });
    qc.invalidateQueries({ queryKey: qk.docChunks(id) });
  };

  const reprocess = useMutation({
    mutationFn: () => post(`/knowledge/${id}/reprocess`, {}),
    onSuccess: () => {
      refresh();
      toast("info", "re-extracting from the stored original");
    },
  });
  const remove = useMutation({
    mutationFn: () => del(`/knowledge/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.docs });
      onClose();
    },
  });

  if (!doc) {
    return (
      <Drawer title="loading…" onClose={onClose}>
        <Empty>fetching the document</Empty>
      </Drawer>
    );
  }

  return (
    <Drawer
      title={doc.title}
      subtitle={
        <span className="font-mono">
          {doc.filename} · {bytes(doc.bytes)} · {doc.chunks} sections
          {doc.extractor && ` · via ${doc.extractor}`}
        </span>
      }
      onClose={onClose}
      actions={
        <>
          <Badge tone={stateTone(doc.state)}>{doc.state}</Badge>
          <Button onClick={() => reprocess.mutate()} disabled={reprocess.isPending} title="re-extract">
            {reprocess.isPending ? <Spinner /> : <RefreshCw className="size-3.5" />}
          </Button>
          <Button variant="danger" onClick={() => remove.mutate()} title="delete">
            <Trash2 className="size-3.5" />
          </Button>
        </>
      }
    >
      {doc.state === "failed" && (
        <div className="rounded-md border border-alert/30 bg-alert/5 p-3 text-[12px] text-alert">
          {doc.error}
          <p className="mt-2 text-ink-500">
            The original is still stored. Fix the cause — usually a missing docling service —
            and re-extract with the button above rather than uploading again.
          </p>
        </div>
      )}
      {doc.state === "pending" && (
        <div className="flex items-center gap-2 rounded-md border border-warn/30 bg-warn/5 p-3 text-[12px] text-warn">
          <Spinner /> extracting; agents cannot search it yet
        </div>
      )}

      <Panel title={`Sections (${chunks?.length ?? 0})`}>
        <p className="mb-3 text-[11px] text-ink-500">
          These are the passages agents retrieve. Each is embedded with its heading, so a clause
          keeps the section title that gives it meaning.
        </p>
        {(chunks?.length ?? 0) === 0 && <Empty>no sections</Empty>}
        <div className="space-y-2">
          {chunks?.map((c) => <ChunkCard key={c.id} chunk={c} />)}
        </div>
      </Panel>
    </Drawer>
  );
}

// ---------------------------------------------------------------- memory

function SharedMemory() {
  const qc = useQueryClient();
  const [limit, setLimit] = useState(PAGE);
  const { data: memories } = useMemories(limit);
  const [query, setQuery] = useState("");
  const [writing, setWriting] = useState(false);
  const search = useMemorySearch();

  const forget = useMutation({
    mutationFn: (id: string) => del(`/memories/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["memories"] }),
  });

  const list: Memory[] = search.data ?? memories ?? [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <Input
          placeholder="search shared memory…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) =>
            e.key === "Enter" && (query.trim() ? search.mutate({ Text: query, Limit: 20 }) : search.reset())
          }
          className="max-w-xl"
        />
        <Button onClick={() => (query.trim() ? search.mutate({ Text: query, Limit: 20 }) : search.reset())}>
          {search.isPending ? <Spinner /> : <Search className="size-3.5" />} search
        </Button>
        {search.data && (
          <Button variant="ghost" onClick={() => { search.reset(); setQuery(""); }}>clear</Button>
        )}
        <Button variant="primary" onClick={() => setWriting(true)}>+ write memory</Button>
      </div>

      <Panel title={search.data ? `${list.length} matches` : `${list.length} memories`}>
        {list.length === 0 && <Empty>Nothing recorded yet.</Empty>}
        <ul className="space-y-2">
          <AnimatePresence initial={false}>
            {list.map((m) => (
              <motion.li
                key={m.id}
                layout
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, height: 0 }}
                className="rounded-md border border-ink-800 bg-ink-850 p-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge tone={m.kind === "pitfall" ? "warn" : m.kind === "decision" ? "good" : "neutral"}>
                        {m.kind}
                      </Badge>
                      <span className="text-[13px] font-medium">{m.title}</span>
                      <span className="text-[11px] text-ink-500">{m.scope}</span>
                      {m.score !== undefined && m.score > 0 && (
                        <span className="font-mono text-[11px] text-ink-500">{m.score.toFixed(3)}</span>
                      )}
                    </div>
                    <p className="mt-1 text-[12px] leading-relaxed whitespace-pre-wrap text-ink-300">
                      {m.body}
                    </p>
                    <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[11px] text-ink-500">
                      <span>{ago(m.created_at)}</span>
                      {m.tags?.map((t) => <Badge key={t}>{t}</Badge>)}
                    </div>
                  </div>
                  <Button variant="danger" onClick={() => forget.mutate(m.id)} title="forget">
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
        {!search.data && (memories?.length ?? 0) >= limit && (
          <div className="mt-3 flex justify-center">
            <Button onClick={() => setLimit(limit + PAGE)}>load more</Button>
          </div>
        )}
      </Panel>

      <AnimatePresence>
        {writing && <WriteMemory onClose={() => setWriting(false)} />}
      </AnimatePresence>
    </div>
  );
}

function WriteMemory({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({ title: "", body: "", kind: "decision", scope: "global" });

  const save = useMutation({
    mutationFn: () =>
      post("/memories", { Title: form.title, Body: form.body, Kind: form.kind, Scope: form.scope }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["memories"] });
      onClose();
    },
  });

  return (
    <Modal title="Write to shared memory" onClose={onClose} wide>
      <Field label="Title" hint="One line naming the thing learned.">
        <Input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
      </Field>
      <Field label="Body" hint="Self-contained. An agent reads this with no other context.">
        <Textarea rows={10} value={form.body} onChange={(e) => setForm({ ...form, body: e.target.value })} />
      </Field>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Kind">
          <Select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
            {MEMORY_KINDS.map((k) => <option key={k} value={k}>{k}</option>)}
          </Select>
        </Field>
        <Field label="Scope">
          <Select value={form.scope} onChange={(e) => setForm({ ...form, scope: e.target.value })}>
            <option value="global">global</option>
            <option value="role">role only</option>
          </Select>
        </Field>
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !form.title.trim() || !form.body.trim()}
        >
          {save.isPending ? <Spinner /> : null} save
        </Button>
      </div>
    </Modal>
  );
}
