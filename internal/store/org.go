package store

import "context"

const roleCols = `id, slug, name, description, system_prompt, model_pref, tool_policy, created_at, updated_at`

func (s *Store) ListRoles(ctx context.Context) ([]Role, error) {
	return selectAll[Role](ctx, s, `SELECT `+roleCols+` FROM roles ORDER BY name`)
}

func (s *Store) GetRole(ctx context.Context, id string) (*Role, error) {
	return selectOne[Role](ctx, s, `SELECT `+roleCols+` FROM roles WHERE id=$1`, id)
}

func (s *Store) GetRoleBySlug(ctx context.Context, slug string) (*Role, error) {
	return selectOne[Role](ctx, s, `SELECT `+roleCols+` FROM roles WHERE slug=$1`, slug)
}

func (s *Store) UpsertRole(ctx context.Context, r Role) (*Role, error) {
	return selectOne[Role](ctx, s, `
		INSERT INTO roles (slug, name, description, system_prompt, model_pref, tool_policy)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (slug) DO UPDATE SET
			name=EXCLUDED.name, description=EXCLUDED.description,
			system_prompt=EXCLUDED.system_prompt, model_pref=EXCLUDED.model_pref,
			tool_policy=EXCLUDED.tool_policy, updated_at=now()
		RETURNING `+roleCols,
		r.Slug, r.Name, r.Description, r.SystemPrompt, r.ModelPref, r.ToolPolicy)
}

func (s *Store) DeleteRole(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM roles WHERE id=$1`, id)
	return err
}

// ---------------------------------------------------------------- skills

const skillCols = `id, slug, version, body, frontmatter, source_path, created_at, updated_at`

func (s *Store) ListSkills(ctx context.Context) ([]Skill, error) {
	return selectAll[Skill](ctx, s, `SELECT `+skillCols+` FROM skills ORDER BY slug`)
}

func (s *Store) GetSkillBySlug(ctx context.Context, slug string) (*Skill, error) {
	return selectOne[Skill](ctx, s, `SELECT `+skillCols+` FROM skills WHERE slug=$1`, slug)
}

func (s *Store) UpsertSkill(ctx context.Context, sk Skill) (*Skill, error) {
	return selectOne[Skill](ctx, s, `
		INSERT INTO skills (slug, version, body, frontmatter, source_path)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (slug) DO UPDATE SET
			version=EXCLUDED.version, body=EXCLUDED.body,
			frontmatter=EXCLUDED.frontmatter, source_path=EXCLUDED.source_path,
			updated_at=now()
		RETURNING `+skillCols,
		sk.Slug, sk.Version, sk.Body, sk.Frontmatter, sk.SourcePath)
}

func (s *Store) DeleteSkill(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM skills WHERE id=$1`, id)
	return err
}

// SkillsForRole returns the role's skills ordered by priority (lowest first).
func (s *Store) SkillsForRole(ctx context.Context, roleID string) ([]Skill, error) {
	return selectAll[Skill](ctx, s, `
		SELECT sk.id, sk.slug, sk.version, sk.body, sk.frontmatter, sk.source_path,
		       sk.created_at, sk.updated_at
		FROM skills sk
		JOIN role_skills rs ON rs.skill_id = sk.id
		WHERE rs.role_id = $1
		ORDER BY rs.priority, sk.slug`, roleID)
}

func (s *Store) AttachSkill(ctx context.Context, roleID, skillID string, priority int) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO role_skills (role_id, skill_id, priority) VALUES ($1,$2,$3)
		ON CONFLICT (role_id, skill_id) DO UPDATE SET priority=EXCLUDED.priority`,
		roleID, skillID, priority)
	return err
}

func (s *Store) DetachSkill(ctx context.Context, roleID, skillID string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM role_skills WHERE role_id=$1 AND skill_id=$2`, roleID, skillID)
	return err
}

// AttachSkillBySlugs resolves both slugs then links them. Used by the skill
// loader so a skill file's `applies_to` frontmatter wires itself up.
func (s *Store) AttachSkillBySlugs(ctx context.Context, roleSlug, skillSlug string, priority int) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO role_skills (role_id, skill_id, priority)
		SELECT r.id, sk.id, $3 FROM roles r, skills sk
		WHERE r.slug=$1 AND sk.slug=$2
		ON CONFLICT (role_id, skill_id) DO UPDATE SET priority=EXCLUDED.priority`,
		roleSlug, skillSlug, priority)
	return err
}

// ---------------------------------------------------------------- tools

const toolCols = `id, kind, display, transport, command, args, env, parse_mode, mcp_style, mcp_target, enabled, created_at`

func (s *Store) ListTools(ctx context.Context) ([]AgentTool, error) {
	return selectAll[AgentTool](ctx, s, `SELECT `+toolCols+` FROM agent_tools ORDER BY display`)
}

func (s *Store) GetTool(ctx context.Context, id string) (*AgentTool, error) {
	return selectOne[AgentTool](ctx, s, `SELECT `+toolCols+` FROM agent_tools WHERE id=$1`, id)
}

func (s *Store) UpdateTool(ctx context.Context, t AgentTool) (*AgentTool, error) {
	return selectOne[AgentTool](ctx, s, `
		UPDATE agent_tools SET display=$2, transport=$3, command=$4, args=$5, env=$6,
			parse_mode=$7, mcp_style=$8, mcp_target=$9, enabled=$10
		WHERE id=$1 RETURNING `+toolCols,
		t.ID, t.Display, t.Transport, t.Command, t.Args, t.Env,
		t.ParseMode, t.MCPStyle, t.MCPTarget, t.Enabled)
}

// ---------------------------------------------------------------- agents

const agentCols = `id, name, role_id, tool_id, llm_mode, model, status, capacity, config, created_at, updated_at`

func (s *Store) ListAgents(ctx context.Context) ([]Agent, error) {
	return selectAll[Agent](ctx, s, `SELECT `+agentCols+` FROM agents ORDER BY name`)
}

func (s *Store) GetAgent(ctx context.Context, id string) (*Agent, error) {
	return selectOne[Agent](ctx, s, `SELECT `+agentCols+` FROM agents WHERE id=$1`, id)
}

func (s *Store) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	return selectOne[Agent](ctx, s, `SELECT `+agentCols+` FROM agents WHERE name=$1`, name)
}

// RosterLine is one agent as another agent sees them: who they are, what they
// own, and whether there is room to give them work.
type RosterLine struct {
	AgentID   string `db:"agent_id" json:"agent_id"`
	AgentName string `db:"agent_name" json:"agent_name"`
	RoleSlug  string `db:"role_slug" json:"role_slug"`
	RoleName  string `db:"role_name" json:"role_name"`
	Status    string `db:"status" json:"status"`
	Capacity  int    `db:"capacity" json:"capacity"`
	ActiveRun int    `db:"active_runs" json:"active_runs"`
	OpenTasks int    `db:"open_tasks" json:"open_tasks"`
}

// Roster answers "who can take this" without exposing model or config, which
// no agent needs and which changes the answer nobody asked for.
func (s *Store) Roster(ctx context.Context) ([]RosterLine, error) {
	return selectAll[RosterLine](ctx, s, `
		SELECT a.id AS agent_id, a.name AS agent_name, r.slug AS role_slug, r.name AS role_name,
		       a.status, a.capacity,
		       (SELECT count(*) FROM runs x
		         WHERE x.agent_id = a.id AND x.state IN ('queued','running'))::int AS active_runs,
		       (SELECT count(*) FROM tasks t
		         WHERE t.assignee_id = a.id AND t.closed_at IS NULL)::int AS open_tasks
		FROM agents a
		JOIN roles r ON r.id = a.role_id
		ORDER BY r.slug, a.name`)
}

func (s *Store) CreateAgent(ctx context.Context, a Agent) (*Agent, error) {
	return selectOne[Agent](ctx, s, `
		INSERT INTO agents (name, role_id, tool_id, llm_mode, model, capacity, config)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+agentCols,
		a.Name, a.RoleID, a.ToolID, orDefault(a.LLMMode, "native"), a.Model,
		orDefaultInt(a.Capacity, 1), a.Config)
}

func (s *Store) UpdateAgent(ctx context.Context, a Agent) (*Agent, error) {
	return selectOne[Agent](ctx, s, `
		UPDATE agents SET name=$2, role_id=$3, tool_id=$4, llm_mode=$5, model=$6,
			status=$7, capacity=$8, config=$9, updated_at=now()
		WHERE id=$1 RETURNING `+agentCols,
		a.ID, a.Name, a.RoleID, a.ToolID, a.LLMMode, a.Model, a.Status, a.Capacity, a.Config)
}

func (s *Store) SetAgentStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE agents SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, id)
	return err
}

// PickAgentForRole returns the idle agent of that role with the fewest active
// runs, or ErrNotFound when every agent of the role is at capacity.
func (s *Store) PickAgentForRole(ctx context.Context, roleID string) (*Agent, error) {
	return selectOne[Agent](ctx, s, `
		SELECT a.id, a.name, a.role_id, a.tool_id, a.llm_mode, a.model, a.status,
		       a.capacity, a.config, a.created_at, a.updated_at
		FROM agents a
		LEFT JOIN runs r ON r.agent_id = a.id AND r.state IN ('queued','running')
		WHERE a.role_id = $1 AND a.status <> 'paused'
		GROUP BY a.id
		HAVING count(r.id) < a.capacity
		ORDER BY count(r.id), a.created_at
		LIMIT 1`, roleID)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
