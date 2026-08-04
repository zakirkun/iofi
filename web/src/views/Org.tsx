import { useState } from "react";
import {
  del, post, put, useFetch,
  type Agent, type Role, type Skill, type Tool,
} from "../lib";
import { Badge, Button, Empty, Field, Input, Modal, Panel, Select, Textarea, stateTone } from "../ui";

export default function OrgView() {
  const { data: roles, reload: reloadRoles } = useFetch<Role[]>("/roles");
  const { data: agents, reload: reloadAgents } = useFetch<Agent[]>("/agents");
  const { data: tools, reload: reloadTools } = useFetch<Tool[]>("/tools");
  const { data: skills, reload: reloadSkills } = useFetch<Skill[]>("/skills");

  const [editRole, setEditRole] = useState<Role | "new" | null>(null);
  const [editSkill, setEditSkill] = useState<Skill | "new" | null>(null);
  const [editAgent, setEditAgent] = useState<Agent | "new" | null>(null);
  const [editTool, setEditTool] = useState<Tool | null>(null);

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Panel
        title="Agents"
        actions={<Button variant="primary" onClick={() => setEditAgent("new")}>+ hire</Button>}
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
                <div className="flex gap-2">
                  <Button onClick={() => setEditAgent(a)}>edit</Button>
                  <Button
                    variant="danger"
                    onClick={async () => { await del(`/agents/${a.id}`); reloadAgents(); }}
                  >
                    fire
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      </Panel>

      <Panel
        title="Roles"
        actions={<Button variant="primary" onClick={() => setEditRole("new")}>+ role</Button>}
      >
        <ul className="divide-y divide-ink-800">
          {roles?.map((r) => (
            <li key={r.id} className="flex items-start justify-between gap-3 py-2.5">
              <div className="min-w-0">
                <div className="text-[13px] font-medium">{r.name}</div>
                <p className="mt-0.5 line-clamp-2 text-[11px] text-ink-500">{r.description}</p>
                <div className="mt-1 flex flex-wrap gap-1">
                  {(r.tool_policy.allow ?? []).slice(0, 6).map((t) => (
                    <Badge key={t}>{t}</Badge>
                  ))}
                </div>
              </div>
              <Button onClick={() => setEditRole(r)}>edit</Button>
            </li>
          ))}
        </ul>
      </Panel>

      <Panel
        title="Skills"
        actions={
          <>
            <Button onClick={async () => { await post("/skills/reload", {}); reloadSkills(); }}>
              reload from disk
            </Button>
            <Button variant="primary" onClick={() => setEditSkill("new")}>+ skill</Button>
          </>
        }
      >
        {(skills?.length ?? 0) === 0 && <Empty>No skills loaded.</Empty>}
        <ul className="divide-y divide-ink-800">
          {skills?.map((s) => (
            <li key={s.id} className="flex items-center justify-between gap-3 py-2.5">
              <div>
                <div className="font-mono text-[13px]">{s.slug}</div>
                <div className="text-[11px] text-ink-500">
                  v{s.version} · {String(s.frontmatter?.description ?? s.source_path ?? "")}
                </div>
              </div>
              <div className="flex gap-2">
                <Button onClick={() => setEditSkill(s)}>edit</Button>
                <Button
                  variant="danger"
                  onClick={async () => { await del(`/skills/${s.id}`); reloadSkills(); }}
                >
                  remove
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </Panel>

      <Panel title="Coding tools">
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
                  <Button
                    onClick={async () => {
                      await put(`/tools/${t.id}`, { ...t, enabled: !t.enabled });
                      reloadTools();
                    }}
                  >
                    {t.enabled ? "disable" : "enable"}
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      </Panel>

      {editRole && (
        <RoleModal
          role={editRole === "new" ? null : editRole}
          skills={skills ?? []}
          onClose={() => setEditRole(null)}
          onSaved={() => { setEditRole(null); reloadRoles(); }}
        />
      )}
      {editSkill && (
        <SkillModal
          skill={editSkill === "new" ? null : editSkill}
          onClose={() => setEditSkill(null)}
          onSaved={() => { setEditSkill(null); reloadSkills(); }}
        />
      )}
      {editAgent && (
        <AgentModal
          agent={editAgent === "new" ? null : editAgent}
          roles={roles ?? []}
          tools={tools ?? []}
          onClose={() => setEditAgent(null)}
          onSaved={() => { setEditAgent(null); reloadAgents(); }}
        />
      )}
      {editTool && (
        <ToolModal
          tool={editTool}
          onClose={() => setEditTool(null)}
          onSaved={() => { setEditTool(null); reloadTools(); }}
        />
      )}
    </div>
  );
}

function ToolModal({
  tool, onClose, onSaved,
}: { tool: Tool; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({ ...tool });
  const [args, setArgs] = useState(JSON.stringify(tool.args ?? [], null, 2));
  const [env, setEnv] = useState(JSON.stringify(tool.env ?? {}, null, 2));
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    try {
      await put(`/tools/${tool.id}`, {
        ...form, args: JSON.parse(args), env: JSON.parse(env),
      });
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

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
          <Select
            value={form.transport}
            onChange={(e) => setForm({ ...form, transport: e.target.value })}
          >
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
      <Field label="Args (JSON array)">
        <Textarea rows={6} value={args} onChange={(e) => setArgs(e.target.value)} />
      </Field>
      <Field label="Env (JSON object)">
        <Textarea rows={3} value={env} onChange={(e) => setEnv(e.target.value)} />
      </Field>
      <div className="grid gap-3 md:grid-cols-3">
        <Field label="Output parsing">
          <Select
            value={form.parse_mode}
            onChange={(e) => setForm({ ...form, parse_mode: e.target.value })}
          >
            <option value="plain">plain</option>
            <option value="jsonl">jsonl</option>
            <option value="claude_stream_json">claude_stream_json</option>
          </Select>
        </Field>
        <Field label="MCP config style">
          <Select
            value={form.mcp_style}
            onChange={(e) => setForm({ ...form, mcp_style: e.target.value })}
          >
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
      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={save}>save tool</Button>
      </div>
    </Modal>
  );
}

function RoleModal({
  role, skills, onClose, onSaved,
}: { role: Role | null; skills: Skill[]; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState<Role>(
    role ?? {
      id: "", slug: "", name: "", description: "", system_prompt: "",
      model_pref: "", tool_policy: { allow: [] },
    },
  );
  const [allow, setAllow] = useState((role?.tool_policy.allow ?? []).join(", "));
  const [error, setError] = useState<string | null>(null);
  const { data: attached, reload } = useFetch<Skill[]>(role ? `/roles/${role.id}/skills` : null);

  const save = async () => {
    try {
      await post("/roles", {
        ...form,
        tool_policy: { allow: allow.split(",").map((s) => s.trim()).filter(Boolean) },
      });
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

  return (
    <Modal title={role ? `Role · ${role.name}` : "New role"} onClose={onClose} wide>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Slug" hint="stable identifier, e.g. tech-leader">
          <Input value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} />
        </Field>
        <Field label="Name">
          <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        </Field>
      </div>
      <Field label="Description" hint="Used by the coordinator when routing work.">
        <Input
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </Field>
      <Field label="System prompt" hint="The persona every agent in this role runs with.">
        <Textarea
          rows={12}
          value={form.system_prompt}
          onChange={(e) => setForm({ ...form, system_prompt: e.target.value })}
        />
      </Field>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Model preference" hint="Blank uses the coordinator default.">
          <Input
            value={form.model_pref}
            onChange={(e) => setForm({ ...form, model_pref: e.target.value })}
          />
        </Field>
        <Field label="Allowed MCP tools" hint="Comma separated. Blank means everything.">
          <Input value={allow} onChange={(e) => setAllow(e.target.value)} />
        </Field>
      </div>

      {role && (
        <Panel title="Attached skills">
          <ul className="mb-3 space-y-1">
            {(attached ?? []).map((s) => (
              <li key={s.id} className="flex items-center justify-between">
                <span className="font-mono text-[12px]">{s.slug}</span>
                <Button
                  variant="ghost"
                  onClick={async () => { await del(`/roles/${role.id}/skills/${s.id}`); reload(); }}
                >
                  detach
                </Button>
              </li>
            ))}
            {(attached?.length ?? 0) === 0 && <Empty>none attached</Empty>}
          </ul>
          <Select
            defaultValue=""
            onChange={async (e) => {
              if (!e.target.value) return;
              await post(`/roles/${role.id}/skills`, { skill_id: e.target.value, priority: 100 });
              e.target.value = "";
              reload();
            }}
          >
            <option value="">attach a skill…</option>
            {skills.map((s) => <option key={s.id} value={s.id}>{s.slug}</option>)}
          </Select>
        </Panel>
      )}

      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={save}>save role</Button>
      </div>
    </Modal>
  );
}

const SKILL_TEMPLATE = `---
slug: my-skill
version: 1
applies_to: [lead-programmer]
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

function SkillModal({
  skill, onClose, onSaved,
}: { skill: Skill | null; onClose: () => void; onSaved: () => void }) {
  const [markdown, setMarkdown] = useState(
    skill ? rebuild(skill) : SKILL_TEMPLATE,
  );
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    try {
      await post("/skills", { markdown });
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

  return (
    <Modal title={skill ? `Skill · ${skill.slug}` : "New skill"} onClose={onClose} wide>
      <p className="text-[12px] text-ink-500">
        Required sections: Purpose, When To Use, Procedure, Constraints, Output Contract, Examples.
        A skill missing one is rejected.
      </p>
      <Textarea rows={24} value={markdown} onChange={(e) => setMarkdown(e.target.value)} />
      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={save}>save skill</Button>
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

function AgentModal({
  agent, roles, tools, onClose, onSaved,
}: {
  agent: Agent | null; roles: Role[]; tools: Tool[];
  onClose: () => void; onSaved: () => void;
}) {
  const [form, setForm] = useState({
    name: agent?.name ?? "",
    role_id: agent?.role_id ?? roles[0]?.id ?? "",
    tool_id: agent?.tool_id ?? tools[0]?.id ?? "",
    llm_mode: agent?.llm_mode ?? "native",
    model: agent?.model ?? "",
    status: agent?.status ?? "idle",
    capacity: agent?.capacity ?? 1,
  });
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    try {
      if (agent) await put(`/agents/${agent.id}`, { ...agent, ...form });
      else await post("/agents", form);
      onSaved();
    } catch (e) {
      setError(String((e as Error).message));
    }
  };

  return (
    <Modal title={agent ? `Agent · ${agent.name}` : "Hire an agent"} onClose={onClose}>
      <Field label="Name" hint="How it signs its work, e.g. Ada.">
        <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
      </Field>
      <Field label="Role">
        <Select value={form.role_id} onChange={(e) => setForm({ ...form, role_id: e.target.value })}>
          {roles.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
        </Select>
      </Field>
      <Field label="Coding tool">
        <Select value={form.tool_id} onChange={(e) => setForm({ ...form, tool_id: e.target.value })}>
          {tools.map((t) => <option key={t.id} value={t.id}>{t.display}</option>)}
        </Select>
      </Field>
      <div className="grid gap-3 md:grid-cols-2">
        <Field
          label="LLM auth"
          hint="native uses the CLI's own login. proxy points it at LiteLLM."
        >
          <Select
            value={form.llm_mode}
            onChange={(e) => setForm({ ...form, llm_mode: e.target.value })}
          >
            <option value="native">native</option>
            <option value="proxy">LiteLLM proxy</option>
          </Select>
        </Field>
        <Field label="Concurrent runs">
          <Input
            type="number"
            min={1}
            value={form.capacity}
            onChange={(e) => setForm({ ...form, capacity: Number(e.target.value) })}
          />
        </Field>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Model" hint="Blank uses the role's preference.">
          <Input value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })} />
        </Field>
        {agent && (
          <Field label="Status" hint="Paused agents are never picked for a task.">
            <Select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
              {["idle", "paused", "busy", "error"].map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </Select>
          </Field>
        )}
      </div>
      {error && <p className="text-[12px] text-alert">{error}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>cancel</Button>
        <Button variant="primary" onClick={save} disabled={!form.name.trim()}>
          {agent ? "save" : "hire"}
        </Button>
      </div>
    </Modal>
  );
}
