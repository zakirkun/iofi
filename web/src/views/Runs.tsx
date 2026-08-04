import { useEffect, useRef, useState } from "react";
import {
  post, useFetch, useStream, ago, cx, money, shortId,
  type Agent, type Run, type RunEvent,
} from "../lib";
import { Badge, Button, Empty, Panel, stateTone } from "../ui";

const PAGE = 50;

export default function RunsView() {
  const [limit, setLimit] = useState(PAGE);
  const { data: runs, reload } = useFetch<Run[]>(`/runs?limit=${limit}`, [limit]);
  const { data: agents } = useFetch<Agent[]>("/agents");
  const [open, setOpen] = useState<string | null>(null);

  useStream(["runs"], reload);

  return (
    <div className="grid gap-4 lg:grid-cols-[24rem_1fr]">
      <Panel title="Runs">
        {(runs?.length ?? 0) === 0 && <Empty>No agent has run yet.</Empty>}
        <ul className="divide-y divide-ink-800">
          {runs?.map((r) => (
            <li key={r.id}>
              <button
                onClick={() => setOpen(r.id)}
                className={cx(
                  "w-full py-2.5 text-left",
                  open === r.id ? "opacity-100" : "opacity-80 hover:opacity-100",
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[13px]">
                    {agents?.find((a) => a.id === r.agent_id)?.name ?? shortId(r.agent_id)}
                  </span>
                  <Badge tone={stateTone(r.state)}>{r.state}</Badge>
                </div>
                <div className="mt-0.5 flex gap-2 font-mono text-[11px] text-ink-500">
                  <span>{ago(r.created_at)}</span>
                  <span>{r.tokens_in + r.tokens_out} tok</span>
                  {r.cost_usd > 0 && <span>{money(r.cost_usd)}</span>}
                </div>
              </button>
            </li>
          ))}
        </ul>
        {(runs?.length ?? 0) >= limit && (
          <div className="mt-3 flex justify-center">
            <Button onClick={() => setLimit(limit + PAGE)}>load more</Button>
          </div>
        )}
      </Panel>

      {open ? <RunLog id={open} /> : <Panel title="Live output"><Empty>Pick a run.</Empty></Panel>}
    </div>
  );
}

function RunLog({ id }: { id: string }) {
  const { data: run, reload: reloadRun } = useFetch<Run>(`/runs/${id}`);
  const { data: initial } = useFetch<RunEvent[]>(`/runs/${id}/events`);
  const [live, setLive] = useState<{ kind: string; text: string }[]>([]);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => setLive([]), [id]);

  useStream([`run:${id}`], (e) => {
    const d = e.data as { text?: string; name?: string };
    setLive((prev) => [...prev, { kind: e.kind, text: d?.text ?? d?.name ?? "" }]);
    if (e.kind === "done" || e.kind === "error") reloadRun();
  });

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [live.length]);

  const history = (initial ?? []).map((e) => ({
    kind: e.kind,
    text: String(e.payload?.text ?? e.payload?.name ?? ""),
  }));
  const all = [...history, ...live];

  return (
    <Panel
      title="Live output"
      actions={
        <>
          {run && <Badge tone={stateTone(run.state)}>{run.state}</Badge>}
          <Button onClick={() => post(`/runs/${id}/cancel`, {})} disabled={run?.state !== "running"}>
            cancel
          </Button>
        </>
      }
    >
      {run?.error && (
        <pre className="mb-3 rounded border border-alert/30 bg-alert/5 p-2 font-mono text-[12px] whitespace-pre-wrap text-alert">
          {run.error}
        </pre>
      )}
      <div className="max-h-[32rem] space-y-1 overflow-y-auto rounded-md bg-ink-950 p-3">
        {all.length === 0 && <Empty>no output yet</Empty>}
        {all.map((e, i) => (
          <div key={i} className="flex gap-2 font-mono text-[12px] leading-relaxed">
            <span
              className={cx(
                "shrink-0 select-none",
                e.kind === "tool_call" && "text-warn",
                e.kind === "error" && "text-alert",
                e.kind === "usage" && "text-ink-500",
                e.kind === "text" && "text-ink-500",
              )}
            >
              {e.kind}
            </span>
            <span className="whitespace-pre-wrap text-ink-100">{e.text}</span>
          </div>
        ))}
        <div ref={endRef} />
      </div>
      {run && (
        <div className="mt-3 flex flex-wrap gap-4 font-mono text-[11px] text-ink-500">
          <span>in {run.tokens_in}</span>
          <span>out {run.tokens_out}</span>
          <span>{money(run.cost_usd)}</span>
          <span>{run.id}</span>
        </div>
      )}
    </Panel>
  );
}
