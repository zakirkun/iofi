import { useState } from "react";
import {
  del, post, useFetch, useStream, ago, cx, money,
  type CostLine, type Dashboard, type Repo, type Workspace,
} from "../lib";
import { Badge, Button, Empty, Field, Input, Panel, Select, Stat, stateTone } from "../ui";

export default function HomeView() {
  const { data, reload } = useFetch<Dashboard>("/dashboard");
  useStream(["runs", "boards", "meetings"], reload);

  if (!data) return <Empty>loading…</Empty>;
  const { agents, roles, tasks, runs, meetings, totals } = data;
  const busy = agents.filter((a) => a.status === "busy").length;

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Agents" value={agents.length} sub={`${busy} working now`} />
        <Stat label="Open tasks" value={totals.open_tasks} sub={`${roles.length} roles defined`} />
        <Stat label="Tokens (last 20 runs)" value={totals.tokens.toLocaleString()} />
        <Stat label="Cost (last 20 runs)" value={money(totals.cost_usd)} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Panel title="Who is working">
          {agents.length === 0 && <Empty>Hire an agent in Org.</Empty>}
          <ul className="divide-y divide-ink-800">
            {agents.map((a) => (
              <li key={a.id} className="flex items-center justify-between py-2">
                <div>
                  <div className="text-[13px]">{a.name}</div>
                  <div className="text-[11px] text-ink-500">
                    {roles.find((r) => r.id === a.role_id)?.name}
                  </div>
                </div>
                <Badge tone={stateTone(a.status)}>{a.status}</Badge>
              </li>
            ))}
          </ul>
        </Panel>

        <Panel title="Recent runs">
          {runs.length === 0 && <Empty>Nothing has run yet.</Empty>}
          <ul className="divide-y divide-ink-800">
            {runs.slice(0, 8).map((r) => (
              <li key={r.id} className="flex items-center justify-between py-2">
                <div className="min-w-0">
                  <div className="truncate text-[13px]">
                    {agents.find((a) => a.id === r.agent_id)?.name ?? "agent"}
                  </div>
                  <div className="font-mono text-[11px] text-ink-500">
                    {ago(r.created_at)} · {r.tokens_in + r.tokens_out} tok
                  </div>
                </div>
                <Badge tone={stateTone(r.state)}>{r.state}</Badge>
              </li>
            ))}
          </ul>
        </Panel>

        <Panel title="Open meetings">
          {meetings.length === 0 && <Empty>No discussion running.</Empty>}
          <ul className="divide-y divide-ink-800">
            {meetings.map((m) => (
              <li key={m.id} className="flex items-center justify-between py-2">
                <div className="min-w-0">
                  <div className="truncate text-[13px]">{m.title}</div>
                  <div className="text-[11px] text-ink-500">
                    {m.kind} · round {m.round}/{m.max_rounds}
                  </div>
                </div>
                <Badge tone={stateTone(m.state)}>{m.state}</Badge>
              </li>
            ))}
          </ul>
        </Panel>

        <Panel title="Board pressure">
          {tasks.length === 0 && <Empty>Board is clear.</Empty>}
          <ul className="space-y-1">
            {Object.entries(
              tasks.reduce<Record<string, number>>((acc, t) => {
                acc[t.column_key] = (acc[t.column_key] ?? 0) + 1;
                return acc;
              }, {}),
            ).map(([col, n]) => (
              <li key={col} className="flex items-center gap-3">
                <span className="w-28 text-[12px] text-ink-300">{col}</span>
                <div className="h-2 flex-1 overflow-hidden rounded bg-ink-800">
                  <div
                    className="h-full bg-signal/60"
                    style={{ width: `${Math.min(100, (n / tasks.length) * 100)}%` }}
                  />
                </div>
                <span className="w-6 text-right font-mono text-[12px] text-ink-500">{n}</span>
              </li>
            ))}
          </ul>
        </Panel>
      </div>

      <Spend />
      <Workspaces />
      <Repos />
    </div>
  );
}

function Repos() {
  const { data: repos, reload } = useFetch<Repo[]>("/repos");
  const [form, setForm] = useState({
    slug: "", remote_url: "", default_branch: "main", webhook_secret: "",
  });
  const [error, setError] = useState<string | null>(null);

  const add = async () => {
    try {
      await post("/repos", form);
      setForm({ slug: "", remote_url: "", default_branch: "main", webhook_secret: "" });
      reload();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

  return (
    <Panel title="Repositories">
      <ul className="divide-y divide-ink-800">
        {repos?.map((r) => (
          <li key={r.id} className="flex items-center justify-between py-2">
            <div>
              <div className="font-mono text-[13px]">{r.slug}</div>
              <div className="text-[11px] text-ink-500">{r.remote_url} · {r.default_branch}</div>
            </div>
            <div className="flex items-center gap-2">
              <Badge tone={r.installed ? "good" : "neutral"}>
                {r.installed ? "cloned" : "not cloned"}
              </Badge>
              <Button
                variant="danger"
                onClick={async () => { await del(`/repos/${r.id}`); reload(); }}
              >
                remove
              </Button>
            </div>
          </li>
        ))}
        {(repos?.length ?? 0) === 0 && <Empty>No repository connected.</Empty>}
      </ul>

      <div className="mt-3 grid gap-3 md:grid-cols-5">
        <Field label="owner/name">
          <Input value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} />
        </Field>
        <Field label="remote url">
          <Input
            value={form.remote_url}
            onChange={(e) => setForm({ ...form, remote_url: e.target.value })}
          />
        </Field>
        <Field label="default branch">
          <Input
            value={form.default_branch}
            onChange={(e) => setForm({ ...form, default_branch: e.target.value })}
          />
        </Field>
        <Field label="webhook secret" hint="Required — unsigned deliveries are refused.">
          <Input
            type="password"
            value={form.webhook_secret}
            onChange={(e) => setForm({ ...form, webhook_secret: e.target.value })}
          />
        </Field>
        <div className="flex items-end">
          <Button variant="primary" onClick={add} disabled={!form.slug || !form.remote_url}>
            connect
          </Button>
        </div>
      </div>
      {error && <p className="mt-2 text-[12px] text-alert">{error}</p>}
      <p className="mt-2 text-[11px] text-ink-500">
        Connecting a repository also creates a board bound to it. Webhook endpoint:{" "}
        <code className="font-mono">POST /webhooks/github</code> — subscribe to issues,
        issue_comment and pull_request, and use the same secret there.
      </p>
    </Panel>
  );
}

function Spend() {
  const [days, setDays] = useState(7);
  const { data: costs } = useFetch<CostLine[]>(`/costs?days=${days}`, [days]);
  const total = (costs ?? []).reduce((n, c) => n + c.cost_usd, 0);

  return (
    <Panel
      title="Spend by agent"
      actions={
        <Select value={String(days)} onChange={(e) => setDays(Number(e.target.value))} className="max-w-28">
          {[1, 7, 30, 90].map((d) => <option key={d} value={d}>{d}d</option>)}
        </Select>
      }
    >
      {(costs?.length ?? 0) === 0 && <Empty>No spend recorded.</Empty>}
      <div className="overflow-x-auto">
        <table className="w-full text-[12px]">
          <thead className="text-ink-500">
            <tr className="text-left">
              <th className="py-1 font-medium">Agent</th>
              <th className="py-1 font-medium">Role</th>
              <th className="py-1 text-right font-medium">Runs</th>
              <th className="py-1 text-right font-medium">Failed</th>
              <th className="py-1 text-right font-medium">Tokens</th>
              <th className="py-1 text-right font-medium">Cost</th>
            </tr>
          </thead>
          <tbody className="font-mono">
            {costs?.map((c) => (
              <tr key={c.agent_id} className="border-t border-ink-800">
                <td className="py-1">{c.agent_name}</td>
                <td className="py-1 text-ink-500">{c.role_slug}</td>
                <td className="py-1 text-right">{c.runs}</td>
                <td className={cx("py-1 text-right", c.failed > 0 && "text-alert")}>{c.failed}</td>
                <td className="py-1 text-right">{(c.tokens_in + c.tokens_out).toLocaleString()}</td>
                <td className="py-1 text-right">{money(c.cost_usd)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-2 text-[11px] text-ink-500">
        Total {money(total)} over {days} days. Actual for Claude Code runs, estimated from the
        rate table for tools that do not report money.
      </p>
    </Panel>
  );
}

function Workspaces() {
  const { data: list, reload } = useFetch<Workspace[]>("/workspaces");
  if ((list?.length ?? 0) === 0) return null;
  return (
    <Panel title="Live workspaces">
      <ul className="divide-y divide-ink-800">
        {list?.map((w) => (
          <li key={w.id} className="flex items-center justify-between gap-3 py-2">
            <div className="min-w-0">
              <div className="truncate font-mono text-[12px]">{w.path}</div>
              <div className="text-[11px] text-ink-500">
                {w.branch || "no branch"} · {ago(w.created_at)}
              </div>
            </div>
            <Button
              onClick={async () => { await post(`/workspaces/${w.id}/release`, {}); reload(); }}
            >
              release
            </Button>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
