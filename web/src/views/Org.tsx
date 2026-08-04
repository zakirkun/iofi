import { useMemo, useState } from "react";
import { useForm } from "@tanstack/react-form";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence } from "motion/react";
import type { ColumnDef } from "@tanstack/react-table";
import { Pencil, Plus, RefreshCw, ShieldAlert, Trash2 } from "lucide-react";
import { cx, del, post, put, type Agent, type Role, type Skill, type Tool, type ToolSpec } from "../lib";
import {
  qk, useAgents, useRoleSkills, useRoles, useSkills, useToolCatalog, useTools,
} from "../queries";
import { toast } from "../store";
import {
  Badge, Button, DataTable, Empty, Field, Input, Modal, Panel, Select, Spinner,
  Textarea, stateTone,
} from "../ui";

export default function OrgView() {
  const { data: roles } = useRoles();
  const { data: agents } = useAgents();
  const { data: tools } = useTools();
  const { data: skills } = useSkills();
  const qc = useQueryClient();

  const [editRole, setEditRole] = useState<Role | "new" | null>(null);
  const [editSkill, setEditSkill] = useState<Skill | "new" | null>(null);
  const [editAgent, setEditAgent] = useState<Agent | "new" | null>(null);
  const [editTool, setEditTool] = useState<Tool | null>(null);
  const [skillFilter, setSkillFilter] = useState("");

  const fire = useMutation({
    mutationFn: (id: string) => del(`/agents/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.agents }),
  });
  const dropSkill = useMutation({
    mutationFn: (id: string) => del(`/skills/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.skills }),
  });
  const reloadSkills = useMutation({
    mutationFn: () => post("/skills/reload", {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.skills });
      toast("good", "skills reloaded from disk");
    },
  });
  const toggleTool = useMutation({
    mutationFn: (t: Tool) => put(`/tools/${t.id}`, { ...t, enabled: !t.enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.tools }),
  });

  const skillColumns = useMemo<ColumnDef<Skill, any>[]>(
    () => [
      {
        header: "Slug",
        accessorKey: "slug",
        cell: (c) => <span className="text-ink-100">{c.getValue<string>()}</span>,
      },
      { header: "v", accessorKey: "version", cell: (c) => <span className="text-ink-500">{c.getValue<number>()}</span> },
      {
        header: "Applies to",
        id: "applies",
        accessorFn: (s) => ((s.frontmatter?.applies_to as string[]) ?? []).join(" "),
        cell: (c) => (
          <div className="flex flex-wrap gap-1">
            {((c.row.original.frontmatter?.applies_to as string[]) ?? []).map((r) => (
              <Badge key={r}>{r}</Badge>
            ))}
          </div>
        ),
      },
      {
        header: "Description",
        id: "description",
        accessorFn: (s) => String(s.frontmatter?.description ?? ""),
        cell: (c) => (
          <span className="font-sans text-ink-500">{c.getValue<string>() || "—"}</span>
        ),
      },
      {
        header: "",
        id: "actions",
        enableSorting: false,
        cell: (c) => (
          <div className="flex justify-end gap-1">
            <Button variant="ghost" onClick={() => setEditSkill(c.row.original)} title="edit">
              <Pencil className="size-3.5" />
            </Button>
            <Button variant="ghost" onClick={() => dropSkill.mutate(c.row.original.id)} title="remove">
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        ),
      },
    ],
    [dropSkill],
  );

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Panel
        title="Agents"
        actions={
          <Button variant="primary" onClick={() => setEditAgent("new")}>
            <Plus className="size-3.5" /> hire
          </Button>
        }
      >
        {(agents?.length ?? 0) === 0 && <Empty>No agents hired yet.</Empty>}
        <ul className="divide-y divide-ink-800">
          {agents?.map((a) => {
            const role = roles?.find((r) => r.id === a.role_id);
            const tool = tools?.find((t) => t.id === a.tool_id);
            return (
              <li key={a.id} className="flex items-center justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-[13px] font-medium">{a.name}</span>
                    <Badge tone={stateTone(a.status)}>{a.status}</Badge>
                  </div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-2 text-[11px] text-ink-500">
                    <span>{role?.name ?? "no role"}</span>
                    <span>·</span>
                    <span>{tool?.display ?? "no tool"}</span>
                    <span>·</span>
                    <span>{a.llm_mode}</span>
                    <span>·</span>
                    <span>capacity {a.capacity}</span>
                  </div>
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" onClick={() => setEditAgent(a)} title="edit">
                    <Pencil className="size-3.5" />
                  </Button>
                  <Button variant="ghost" onClick={() => fire.mutate(a.id)} title="fire">
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      </Panel>

      <Panel
        title="Roles"
        actions={
          <Button variant="primary" onClick={() => setEditRole("new")}>
            <Plus className="size-3.5" /> role
          </Button>
        }
      >
        <ul className="divide-y divide-ink-800">
          {roles?.map((r) => {
            const allowed = r.tool_policy.allow ?? [];
            return (
              <li key={r.id} className="flex items-start justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  <div className="text-[13px] font-medium">{r.name}</div>
                  <p className="mt-0.5 line-clamp-2 text-[11px] text-ink-500">{r.description}</p>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {allowed.slice(0, 6).map((t) => <Badge key={t}>{t}</Badge>)}
                    {allowed.length > 6 && <Badge>+{allowed.length - 6}</Badge>}
                    {allowed.length === 0 && <Badge tone="warn">every tool</Badge>}
                  </div>
                </div>
                <Button variant="ghost" onClick={() => setEditRole(r)} title="edit">
                  <Pencil className="size-3.5" />
                </Button>
              </li>
            );
          })}
        </ul>
      </Panel>

      <Panel
        title={`Skills (${skills?.length ?? 0})`}
        className="xl:col-span-2"
        actions={
          <>
            <Input
              placeholder="filter…"
              value={skillFilter}
              onChange={(e) => setSkillFilter(e.target.value)}
              className="max-w-48"
            />
            <Button onClick={() => reloadSkills.mutate()} disabled={reloadSkills.isPending}>
              {reloadSkills.isPending ? <Spinner /> : <RefreshCw className="size-3.5" />} reload from disk
            </Button>
            <Button variant="primary" onClick={() => setEditSkill("new")}>
              <Plus className="size-3.5" /> skill
            </Button>
          </>
        }
      >
        <DataTable
          data={skills ?? []}
          columns={skillColumns}
          filter={skillFilter}
          empty="No skills loaded."
          initialSort={[{ id: "slug", desc: false }]}
        />
      </Panel>

      <Panel title="Coding tools" className="xl:col-span-2">
        <p className="mb-2 text-[11px] text-ink-500">
          A new CLI is a row here, not new code. Templates available in args and env:
          <code className="font-mono"> {"{{.Prompt}} {{.Workspace}} {{.MCPConfig}} {{.Model}} {{.RunID}}"}</code>
        </p>
        <ul className="divide-y divide-ink-800">
          {tools?.map((t) => (
            <li key={t.id} className="py-2.5">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-[13px]">
                    {t.display}
                    <Badge tone={t.enabled ? "good" : "neutral"}>
                      {t.enabled ? "enabled" : "disabled"}
                    </Badge>
                    <Badge>{t.transport}</Badge>
                  </div>
                  <div className="mt-0.5 truncate font-mono text-[11px] text-ink-500">
                    {t.command} {t.args?.join(" ")}
                  </div>
                </div>
                <div className="flex gap-2">
                  <Button onClick={() => setEditTool(t)}>edit</Button>
                  <Button onClick={() => toggleTool.mutate(t)}>
                    {t.enabled ? "disable" : "enable"}
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      </Panel>

      <AnimatePresence>
        {editRole && (
          <RoleModal
            role={editRole === "new" ? null : editRole}
            skills={skills ?? []}
            onClose={() => setEditRole(null)}
          />
        )}
        {editSkill && (
          <SkillModal
            skill={editSkill === "new" ? null : editSkill}
            onClose={() => setEditSkill(null)}
          />
        )}
        {editAgent && (
          <AgentModal
            agent={editAgent === "new" ? null : editAgent}
            roles={roles ?? []}
            tools={tools ?? []}
            onClose={() => setEditAgent(null)}
          />
        )}
        {editTool && <ToolModal tool={editTool} onClose={() => setEditTool(null)} />}
      </AnimatePresence>
    </div>
  );
}

// ---------------------------------------------------------------- tool policy

const CATEGORY_LABEL: Record<string, string> = {
  board: "Board and tasks",
  knowledge: "Memory and documents",
  org: "Who exists",
  collab: "Talking to others",
  repo: "Repository",
  ops: "Runs and cost",
};

/** Checkboxes generated from /tools/catalog. The previous editor was a
 *  comma-separated text field, which let the founder grant a tool that does not
 *  exist — accepted silently, then simply absent when an agent went looking. */
function ToolPolicyEditor({
  allow,
  onChange,
}: {
  allow: string[];
  onChange: (next: string[]) => void;
}) {
  const { data, isLoading } = useToolCatalog();
  const toggle = (name: string) =>
    onChange(allow.includes(name) ? allow.filter((t) => t !== name) : [...allow, name]);

  if (isLoading) return <Empty>loading the tool catalog…</Empty>;
  const categories = data?.categories ?? {};

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2 text-[11px] text-ink-500">
        <span>{allow.length} of {data?.tools.length ?? 0} granted.</span>
        <Button variant="ghost" onClick={() => onChange((data?.tools ?? []).map((t) => t.name))}>
          all
        </Button>
        <Button variant="ghost" onClick={() => onChange([])}>none</Button>
        <span>An empty list means every tool, which is rarely what you want.</span>
      </div>
      {Object.entries(categories).map(([cat, specs]) => (
        <div key={cat}>
          <div className="mb-1 text-[11px] font-medium tracking-wide text-ink-300 uppercase">
            {CATEGORY_LABEL[cat] ?? cat}
          </div>
          <div className="grid gap-1 sm:grid-cols-2">
            {(specs as ToolSpec[]).map((spec) => {
              const on = allow.includes(spec.name);
              return (
                <label
                  key={spec.name}
                  title={spec.description}
                  className={cx(
                    "flex cursor-pointer items-start gap-2 rounded-md border px-2 py-1.5 text-[12px] transition-colors",
                    on ? "border-signal/30 bg-signal/5" : "border-ink-800 bg-ink-950 hover:border-ink-700",
                  )}
                >
                  <input
                    type="checkbox"
                    checked={on}
                    onChange={() => toggle(spec.name)}
                    className="mt-0.5"
                  />
                  <span className="min-w-0">
                    <span className="flex items-center gap-1 font-mono text-ink-100">
                      {spec.name}
                      {spec.mutates && (
                        <ShieldAlert className="size-3 text-warn" aria-label="changes company state" />
                      )}
                    </span>
                    <span className="line-clamp-2 text-[11px] text-ink-500">{spec.description}</span>
                  </span>
                </label>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

function RoleModal({
  role, skills, onClose,
}: { role: Role | null; skills: Skill[]; onClose: () => void }) {
  const qc = useQueryClient();
  const { data: attached } = useRoleSkills(role?.id ?? null);
  const [allow, setAllow] = useState(role?.tool_policy.allow ?? []);

  const form = useForm({
    defaultValues: {
      slug: role?.slug ?? "",
      name: role?.name ?? "",
      description: role?.description ?? "",
      system_prompt: role?.system_prompt ?? "",
      model_pref: role?.model_pref ?? "",
    },
    onSubmit: async ({ value }) => {
      await post("/roles", { ...role, ...value, tool_policy: { allow } });
      qc.invalidateQueries({ queryKey: qk.roles });
      onClose();
    },
  });

  const detach = useMutation({
    mutationFn: (skillID: string) => del(`/roles/${role!.id}/skills/${skillID}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.roleSkills(role!.id) }),
  });
  const attach = useMutation({
    mutationFn: (skillID: string) => post(`/roles/${role!.id}/skills`, { skill_id: skillID, priority: 100 }),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.roleSkills(role!.id) }),
  });

  return (
    <Modal title={role ? `Role · ${role.name}` : "New role"} onClose={onClose} wide>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          form.handleSubmit();
        }}
        className="space-y-4"
      >
        <div className="grid gap-3 md:grid-cols-2">
          <form.Field
            name="slug"
            validators={{
              onChange: ({ value }) =>
                /^[a-z0-9-]+$/.test(value) ? undefined : "lowercase letters, digits and dashes only",
            }}
          >
            {(field) => (
              <Field label="Slug" hint={field.state.meta.errors[0] ?? "stable identifier, e.g. tech-leader"}>
                <Input
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  className={cx(field.state.meta.errors.length > 0 && "border-alert/50")}
                />
              </Field>
            )}
          </form.Field>
          <form.Field name="name" validators={{ onChange: ({ value }) => (value.trim() ? undefined : "required") }}>
            {(field) => (
              <Field label="Name" hint={field.state.meta.errors[0]}>
                <Input value={field.state.value} onChange={(e) => field.handleChange(e.target.value)} />
              </Field>
            )}
          </form.Field>
        </div>

        <form.Field name="description">
          {(field) => (
            <Field label="Description" hint="Used by the coordinator when routing work.">
              <Input value={field.state.value} onChange={(e) => field.handleChange(e.target.value)} />
            </Field>
          )}
        </form.Field>

        <form.Field name="system_prompt">
          {(field) => (
            <Field label="System prompt" hint="The persona every agent in this role runs with.">
              <Textarea
                rows={12}
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="model_pref">
          {(field) => (
            <Field label="Model preference" hint="Blank uses the coordinator default.">
              <Input value={field.state.value} onChange={(e) => field.handleChange(e.target.value)} />
            </Field>
          )}
        </form.Field>

        <Panel title="Allowed MCP tools">
          <ToolPolicyEditor allow={allow} onChange={setAllow} />
        </Panel>

        {role && (
          <Panel title="Attached skills">
            <ul className="mb-3 space-y-1">
              {(attached ?? []).map((s) => (
                <li key={s.id} className="flex items-center justify-between">
                  <span className="font-mono text-[12px]">{s.slug}</span>
                  <Button variant="ghost" onClick={() => detach.mutate(s.id)}>detach</Button>
                </li>
              ))}
              {(attached?.length ?? 0) === 0 && <Empty>none attached</Empty>}
            </ul>
            <Select
              value=""
              onChange={(e) => e.target.value && attach.mutate(e.target.value)}
            >
              <option value="">attach a skill…</option>
              {skills
                .filter((s) => !(attached ?? []).some((a) => a.id === s.id))
                .map((s) => <option key={s.id} value={s.id}>{s.slug}</option>)}
            </Select>
          </Panel>
        )}

        <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting] as const}>
          {([canSubmit, isSubmitting]) => (
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={onClose}>cancel</Button>
              <Button type="submit" variant="primary" disabled={!canSubmit || isSubmitting}>
                {isSubmitting ? <Spinner /> : null} save role
              </Button>
            </div>
          )}
        </form.Subscribe>
      </form>
    </Modal>
  );
}

function AgentModal({
  agent, roles, tools, onClose,
}: { agent: Agent | null; roles: Role[]; tools: Tool[]; onClose: () => void }) {
  const qc = useQueryClient();

  const form = useForm({
    defaultValues: {
      name: agent?.name ?? "",
      role_id: agent?.role_id ?? roles[0]?.id ?? "",
      tool_id: agent?.tool_id ?? tools[0]?.id ?? "",
      llm_mode: agent?.llm_mode ?? "native",
      model: agent?.model ?? "",
      status: agent?.status ?? "idle",
      capacity: agent?.capacity ?? 1,
    },
    onSubmit: async ({ value }) => {
      if (agent) await put(`/agents/${agent.id}`, { ...agent, ...value });
      else await post("/agents", value);
      qc.invalidateQueries({ queryKey: qk.agents });
      onClose();
    },
  });

  return (
    <Modal title={agent ? `Agent · ${agent.name}` : "Hire an agent"} onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          form.handleSubmit();
        }}
        className="space-y-4"
      >
        <form.Field name="name" validators={{ onChange: ({ value }) => (value.trim() ? undefined : "required") }}>
          {(field) => (
            <Field label="Name" hint={field.state.meta.errors[0] ?? "How it signs its work, e.g. Ada."}>
              <Input
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.target.value)}
              />
            </Field>
          )}
        </form.Field>

        <form.Field name="role_id">
          {(field) => (
            <Field label="Role">
              <Select value={field.state.value} onChange={(e) => field.handleChange(e.target.value)}>
                {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
              </Select>
            </Field>
          )}
        </form.Field>

        <form.Field name="tool_id">
          {(field) => (
            <Field label="Coding tool">
              <Select value={field.state.value} onChange={(e) => field.handleChange(e.target.value)}>
                {tools.map((t) => <option key={t.id} value={t.id}>{t.display}</option>)}
              </Select>
            </Field>
          )}
        </form.Field>

        <div className="grid gap-3 md:grid-cols-2">
          <form.Field name="llm_mode">
            {(field) => (
              <Field label="LLM auth" hint="native uses the CLI's own login. proxy points it at LiteLLM.">
                <Select value={field.state.value} onChange={(e) => field.handleChange(e.target.value)}>
                  <option value="native">native</option>
                  <option value="proxy">LiteLLM proxy</option>
                </Select>
              </Field>
            )}
          </form.Field>
          <form.Field
            name="capacity"
            validators={{ onChange: ({ value }) => (value >= 1 ? undefined : "at least 1") }}
          >
            {(field) => (
              <Field label="Concurrent runs" hint={field.state.meta.errors[0]}>
                <Input
                  type="number"
                  min={1}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(Number(e.target.value))}
                />
              </Field>
            )}
          </form.Field>
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          <form.Field name="model">
            {(field) => (
              <Field label="Model" hint="Blank uses the role's preference.">
                <Input value={field.state.value} onChange={(e) => field.handleChange(e.target.value)} />
              </Field>
            )}
          </form.Field>
          {agent && (
            <form.Field name="status">
              {(field) => (
                <Field label="Status" hint="Paused agents are never picked for a task.">
                  <Select value={field.state.value} onChange={(e) => field.handleChange(e.target.value)}>
                    {["idle", "paused", "busy", "error"].map((s) => (
                      <option key={s} value={s}>{s}</option>
                    ))}
                  </Select>
                </Field>
              )}
            </form.Field>
          )}
        </div>

        <form.Subscribe selector={(s) => [s.canSubmit, s.isSubmitting] as const}>
          {([canSubmit, isSubmitting]) => (
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={onClose}>cancel</Button>
              <Button type="submit" variant="primary" disabled={!canSubmit || isSubmitting}>
                {isSubmitting ? <Spinner /> : null} {agent ? "save" : "hire"}
              </Button>
            </div>
          )}
        </form.Subscribe>
      </form>
    </Modal>
  );
}

function ToolModal({ tool, onClose }: { tool: Tool; onClose: () => void }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({ ...tool });
  const [args, setArgs] = useState(JSON.stringify(tool.args ?? [], null, 2));
  const [env, setEnv] = useState(JSON.stringify(tool.env ?? {}, null, 2));

  const save = useMutation({
    mutationFn: () =>
      put(`/tools/${tool.id}`, { ...form, args: JSON.parse(args), env: JSON.parse(env) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.tools });
      onClose();
    },
  });

  const jsonOK = (s: string) => {
    try {
      JSON.parse(s);
      return true;
    } catch {
      return false;
    }
  };
  const valid = jsonOK(args) && jsonOK(env);

  return (
    <Modal title={`Tool · ${tool.kind}`} onClose={onClose} wide>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Display name">
          <Input value={form.display} onChange={(e) => setForm({ ...form, display: e.target.value })} />
        </Field>
        <Field
          label="Transport"
          hint="spawn runs the CLI headless. mcp_client connects to a tool that exposes an agent turn as an MCP tool."
        >
          <Select value={form.transport} onChange={(e) => setForm({ ...form, transport: e.target.value })}>
            <option value="spawn">spawn</option>
            <option value="mcp_client">mcp_client</option>
          </Select>
        </Field>
      </div>
      <Field label="Command">
        <Input
          value={form.command}
          onChange={(e) => setForm({ ...form, command: e.target.value })}
          className="font-mono"
        />
      </Field>
      <Field label="Args (JSON array)" hint={jsonOK(args) ? undefined : "not valid JSON"}>
        <Textarea
          rows={6}
          value={args}
          onChange={(e) => setArgs(e.target.value)}
          className={cx(!jsonOK(args) && "border-alert/50")}
        />
      </Field>
      <Field label="Env (JSON object)" hint={jsonOK(env) ? undefined : "not valid JSON"}>
        <Textarea
          rows={3}
          value={env}
          onChange={(e) => setEnv(e.target.value)}
          className={cx(!jsonOK(env) && "border-alert/50")}
        />
      </Field>
      <div className="grid gap-3 md:grid-cols-3">
        <Field label="Output parsing">
          <Select value={form.parse_mode} onChange={(e) => setForm({ ...form, parse_mode: e.target.value })}>
            <option value="plain">plain</option>
            <option value="jsonl">jsonl</option>
            <option value="claude_stream_json">claude_stream_json</option>
          </Select>
        </Field>
        <Field label="MCP config style">
          <Select value={form.mcp_style} onChange={(e) => setForm({ ...form, mcp_style: e.target.value })}>
            <option value="flag">flag</option>
            <option value="config_file">config_file</option>
            <option value="env">env</option>
          </Select>
        </Field>
        <Field label="MCP target" hint="Flag name, config path, or remote tool name.">
          <Input
            value={form.mcp_target}
            onChange={(e) => setForm({ ...form, mcp_target: e.target.value })}
            className="font-mono"
          />
        </Field>
      </div>
      <label className="flex items-center gap-2 text-[13px] text-ink-300">
        <input
          type="checkbox"
          checked={form.enabled}
          onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
        />
        enabled
      </label>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={() => save.mutate()} disabled={!valid || save.isPending}>
          {save.isPending ? <Spinner /> : null} save tool
        </Button>
      </div>
    </Modal>
  );
}

const SKILL_TEMPLATE = `---
slug: my-skill
version: 1
applies_to: [lead-programmer]
tools_required: [memory_search, task_update]
priority: 100
description: one line, used when the coordinator picks skills
---
## Purpose

## When To Use

## Procedure

## Constraints

## Output Contract

## Examples
`;

function SkillModal({ skill, onClose }: { skill: Skill | null; onClose: () => void }) {
  const qc = useQueryClient();
  const [markdown, setMarkdown] = useState(skill ? rebuild(skill) : SKILL_TEMPLATE);

  const save = useMutation({
    mutationFn: () => post("/skills", { markdown }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.skills });
      onClose();
    },
  });

  return (
    <Modal title={skill ? `Skill · ${skill.slug}` : "New skill"} onClose={onClose} wide>
      <p className="text-[12px] text-ink-500">
        Required sections: Purpose, When To Use, Procedure, Constraints, Output Contract, Examples.
        A skill missing one is rejected, and so is one whose <code className="font-mono">tools_required</code>{" "}
        names a tool that does not exist.
      </p>
      <Textarea rows={24} value={markdown} onChange={(e) => setMarkdown(e.target.value)} />
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending ? <Spinner /> : null} save skill
        </Button>
      </div>
    </Modal>
  );
}

/** The API stores frontmatter and body apart; the editor round-trips one file. */
function rebuild(s: Skill): string {
  const fm = s.frontmatter ?? {};
  const lines = Object.entries(fm)
    .filter(([, v]) => v !== null && v !== undefined && v !== "")
    .map(([k, v]) => `${k}: ${Array.isArray(v) ? `[${v.join(", ")}]` : v}`);
  return `---\n${lines.join("\n")}\n---\n${s.body}\n`;
}
