import { useState } from "react";
import { del, post, useFetch, ago, type Memory } from "../lib";
import { Badge, Button, Empty, Field, Input, Modal, Panel, Select, Textarea } from "../ui";

const KINDS = ["fact", "decision", "convention", "pitfall", "artifact"];

const PAGE = 50;

export default function KnowledgeView() {
  const [limit, setLimit] = useState(PAGE);
  const { data: memories, reload } = useFetch<Memory[]>(`/memories?limit=${limit}`, [limit]);
  const [results, setResults] = useState<Memory[] | null>(null);
  const [query, setQuery] = useState("");
  const [writing, setWriting] = useState(false);

  const search = async () => {
    if (!query.trim()) return setResults(null);
    setResults(await post<Memory[]>("/memories/search", { Text: query, Limit: 20 }));
  };

  const list = results ?? memories ?? [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <Input
          placeholder="search shared memory…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && search()}
          className="max-w-xl"
        />
        <Button onClick={search}>search</Button>
        {results && <Button variant="ghost" onClick={() => { setResults(null); setQuery(""); }}>clear</Button>}
        <Button variant="primary" onClick={() => setWriting(true)}>+ write memory</Button>
      </div>

      <Panel title={results ? `${list.length} matches` : `${list.length} memories`}>
        {list.length === 0 && <Empty>Nothing recorded yet.</Empty>}
        <ul className="space-y-2">
          {list.map((m) => (
            <li key={m.id} className="rounded-md border border-ink-800 bg-ink-850 p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge tone={m.kind === "pitfall" ? "warn" : m.kind === "decision" ? "good" : "neutral"}>
                      {m.kind}
                    </Badge>
                    <span className="text-[13px] font-medium">{m.title}</span>
                    <span className="text-[11px] text-ink-500">{m.scope}</span>
                    {m.score !== undefined && m.score > 0 && (
                      <span className="font-mono text-[11px] text-ink-500">
                        {m.score.toFixed(3)}
                      </span>
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
                <Button
                  variant="danger"
                  onClick={async () => { await del(`/memories/${m.id}`); reload(); }}
                >
                  forget
                </Button>
              </div>
            </li>
          ))}
        </ul>
        {!results && (memories?.length ?? 0) >= limit && (
          <div className="mt-3 flex justify-center">
            <Button onClick={() => setLimit(limit + PAGE)}>load more</Button>
          </div>
        )}
      </Panel>

      {writing && (
        <WriteModal onClose={() => setWriting(false)} onSaved={() => { setWriting(false); reload(); }} />
      )}
    </div>
  );
}

function WriteModal({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({ title: "", body: "", kind: "decision", scope: "global" });
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    try {
      await post("/memories", { Title: form.title, Body: form.body, Kind: form.kind, Scope: form.scope });
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

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
            {KINDS.map((k) => <option key={k} value={k}>{k}</option>)}
          </Select>
        </Field>
        <Field label="Scope">
          <Select value={form.scope} onChange={(e) => setForm({ ...form, scope: e.target.value })}>
            <option value="global">global</option>
            <option value="role">role only</option>
          </Select>
        </Field>
      </div>
      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={save} disabled={!form.title.trim() || !form.body.trim()}>
          save
        </Button>
      </div>
    </Modal>
  );
}
