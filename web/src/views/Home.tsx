import { useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { motion } from "motion/react";
import type { ColumnDef } from "@tanstack/react-table";
import { GitBranch, Plug, Trash2 } from "lucide-react";
import { ago, cx, del, money, post, type CostLine } from "../lib";
import {
  qk, useCosts, useDashboard, useRepos, useWorkspaces,
} from "../queries";
import { Badge, Button, DataTable, Empty, Field, Input, Panel, Select, Spinner, Stat, stateTone } from "../ui";

export default function HomeView() {
  const { data } = useDashboard();

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
                  <motion.div
                    className="h-full bg-signal/60"
                    initial={{ width: 0 }}
                    animate={{ width: `${Math.min(100, (n / tasks.length) * 100)}%` }}
                    transition={{ type: "spring", stiffness: 160, damping: 24 }}
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
  const qc = useQueryClient();
  const { data: repos } = useRepos();
  const [form, setForm] = useState({
    slug: "", remote_url: "", default_branch: "main", webhook_secret: "",
  });

  const done = () => qc.invalidateQueries({ queryKey: qk.repos });

  const add = useMutation({
    mutationFn: () => post("/repos", form),
    onSuccess: () => {
      setForm({ slug: "", remote_url: "", default_branch: "main", webhook_secret: "" });
      done();
      qc.invalidateQueries({ queryKey: qk.boards });
    },
  });
  const remove = useMutation({ mutationFn: (id: string) => del(`/repos/${id}`), onSuccess: done });

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
                <GitBranch className="size-3" />
                {r.installed ? "cloned" : "not cloned"}
              </Badge>
              <Button variant="ghost" onClick={() => remove.mutate(r.id)} title="remove">
                <Trash2 className="size-3.5" />
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
          <Button
            variant="primary"
            onClick={() => add.mutate()}
            disabled={add.isPending || !form.slug || !form.remote_url}
          >
            {add.isPending ? <Spinner /> : <Plug className="size-3.5" />} connect
          </Button>
        </div>
      </div>
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
  const { data: costs } = useCosts(days);
  const total = (costs ?? []).reduce((n, c) => n + c.cost_usd, 0);

  const columns = useMemo<ColumnDef<CostLine, any>[]>(
    () => [
      { header: "Agent", accessorKey: "agent_name" },
      {
        header: "Role",
        accessorKey: "role_slug",
        cell: (c) => <span className="text-ink-500">{c.getValue<string>()}</span>,
      },
      { header: "Runs", accessorKey: "runs", cell: (c) => <span className="block text-right">{c.getValue<number>()}</span> },
      {
        header: "Failed",
        accessorKey: "failed",
        cell: (c) => (
          <span className={cx("block text-right", c.getValue<number>() > 0 && "text-alert")}>
            {c.getValue<number>()}
          </span>
        ),
      },
      {
        header: "Tokens",
        id: "tokens",
        accessorFn: (r) => r.tokens_in + r.tokens_out,
        cell: (c) => <span className="block text-right">{c.getValue<number>().toLocaleString()}</span>,
      },
      {
        header: "Cost",
        accessorKey: "cost_usd",
        cell: (c) => <span className="block text-right">{money(c.getValue<number>())}</span>,
      },
    ],
    [],
  );

  return (
    <Panel
      title="Spend by agent"
      actions={
        <Select value={String(days)} onChange={(e) => setDays(Number(e.target.value))} className="max-w-28">
          {[1, 7, 30, 90].map((d) => <option key={d} value={d}>{d}d</option>)}
        </Select>
      }
    >
      <DataTable
        data={costs ?? []}
        columns={columns}
        empty="No spend recorded."
        initialSort={[{ id: "cost_usd", desc: true }]}
      />
      <p className="mt-2 text-[11px] text-ink-500">
        Total {money(total)} over {days} days. Actual for Claude Code runs, estimated from the
        rate table for tools that do not report money.
      </p>
    </Panel>
  );
}

function Workspaces() {
  const qc = useQueryClient();
  const { data: list } = useWorkspaces();
  const release = useMutation({
    mutationFn: (id: string) => post(`/workspaces/${id}/release`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.workspaces }),
  });

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
            <Button onClick={() => release.mutate(w.id)} disabled={release.isPending}>release</Button>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
