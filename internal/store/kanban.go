package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const boardCols = `id, slug, name, repo_id, columns, created_at`

func (s *Store) ListBoards(ctx context.Context) ([]Board, error) {
	return selectAll[Board](ctx, s, `SELECT `+boardCols+` FROM boards ORDER BY name`)
}

func (s *Store) GetBoard(ctx context.Context, id string) (*Board, error) {
	return selectOne[Board](ctx, s, `SELECT `+boardCols+` FROM boards WHERE id=$1`, id)
}

func (s *Store) GetBoardBySlug(ctx context.Context, slug string) (*Board, error) {
	return selectOne[Board](ctx, s, `SELECT `+boardCols+` FROM boards WHERE slug=$1`, slug)
}

func (s *Store) BoardForRepo(ctx context.Context, repoID string) (*Board, error) {
	return selectOne[Board](ctx, s,
		`SELECT `+boardCols+` FROM boards WHERE repo_id=$1 ORDER BY created_at LIMIT 1`, repoID)
}

// DefaultColumns is the board every new repository starts with.
func DefaultColumns() Columns {
	return Columns{
		{Key: "backlog", Name: "Backlog"},
		{Key: "ready", Name: "Ready"},
		{Key: "in_progress", Name: "In Progress", WIP: 5},
		{Key: "review", Name: "Review"},
		{Key: "qa", Name: "QA"},
		{Key: "done", Name: "Done"},
		{Key: "blocked", Name: "Blocked"},
	}
}

func (s *Store) CreateBoard(ctx context.Context, b Board) (*Board, error) {
	return selectOne[Board](ctx, s, `
		INSERT INTO boards (slug, name, repo_id, columns) VALUES ($1,$2,$3,$4)
		RETURNING `+boardCols, b.Slug, b.Name, b.RepoID, b.Columns)
}

func (s *Store) UpdateBoard(ctx context.Context, b Board) (*Board, error) {
	return selectOne[Board](ctx, s, `
		UPDATE boards SET name=$2, repo_id=$3, columns=$4 WHERE id=$1
		RETURNING `+boardCols, b.ID, b.Name, b.RepoID, b.Columns)
}

// ---------------------------------------------------------------- tasks

const taskCols = `id, board_id, parent_id, seq, title, spec, column_key, position, priority,
	estimate, role_required, assignee_id, depends_on, labels, acceptance, created_by,
	created_at, updated_at, closed_at`

type TaskFilter struct {
	BoardID   string
	ColumnKey string
	RoleID    string
	AgentID   string
	Open      bool // exclude done/closed
	Limit     int
}

func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]Task, error) {
	var where []string
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.BoardID != "" {
		add("board_id=$%d", f.BoardID)
	}
	if f.ColumnKey != "" {
		add("column_key=$%d", f.ColumnKey)
	}
	if f.RoleID != "" {
		add("role_required=$%d", f.RoleID)
	}
	if f.AgentID != "" {
		add("assignee_id=$%d", f.AgentID)
	}
	if f.Open {
		where = append(where, "closed_at IS NULL")
	}
	sql := `SELECT ` + taskCols + ` FROM tasks`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += " ORDER BY priority, position, created_at"
	if f.Limit > 0 {
		args = append(args, f.Limit)
		sql += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	return selectAll[Task](ctx, s, sql, args...)
}

func (s *Store) GetTask(ctx context.Context, id string) (*Task, error) {
	return selectOne[Task](ctx, s, `SELECT `+taskCols+` FROM tasks WHERE id=$1`, id)
}

// SearchTasks matches text across the three fields that carry meaning. Closed
// tasks are included by default: the question an agent asks before creating
// work is "has this been done", and excluding finished work answers it wrongly.
func (s *Store) SearchTasks(ctx context.Context, boardID, query string, openOnly bool, limit int) ([]Task, error) {
	where := []string{`(title ILIKE '%'||$1||'%' OR spec ILIKE '%'||$1||'%' OR acceptance ILIKE '%'||$1||'%')`}
	args := []any{query}
	if boardID != "" {
		args = append(args, boardID)
		where = append(where, fmt.Sprintf("board_id=$%d", len(args)))
	}
	if openOnly {
		where = append(where, "closed_at IS NULL")
	}
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit)
	return selectAll[Task](ctx, s, `SELECT `+taskCols+` FROM tasks WHERE `+
		strings.Join(where, " AND ")+
		fmt.Sprintf(` ORDER BY closed_at IS NOT NULL, priority, updated_at DESC LIMIT $%d`, len(args)),
		args...)
}

func (s *Store) CreateTask(ctx context.Context, t Task, actor string) (*Task, error) {
	created, err := selectOne[Task](ctx, s, `
		INSERT INTO tasks (board_id, parent_id, title, spec, column_key, position, priority,
			estimate, role_required, assignee_id, depends_on, labels, acceptance, created_by)
		VALUES ($1,$2,$3,$4,COALESCE(NULLIF($5,''),'backlog'),
			COALESCE($6, (SELECT COALESCE(max(position),0)+1000 FROM tasks WHERE board_id=$1)),
			$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+taskCols,
		t.BoardID, t.ParentID, t.Title, t.Spec, t.ColumnKey, nilIfZero(t.Position),
		orDefaultInt(t.Priority, 3), t.Estimate, t.RoleRequired, t.AssigneeID,
		nonNil(t.DependsOn), nonNil(t.Labels), t.Acceptance, orDefault(actor, "founder"))
	if err != nil {
		return nil, err
	}
	_ = s.AddTaskEvent(ctx, created.ID, actor, "created", JSONMap{"title": created.Title})
	return created, nil
}

type TaskPatch struct {
	Title        *string
	Spec         *string
	ColumnKey    *string
	Position     *float64
	Priority     *int
	Estimate     *float64
	RoleRequired *string
	AssigneeID   *string
	Acceptance   *string
	Labels       *[]string
	DependsOn    *[]string
	Close        *bool
}

func (s *Store) UpdateTask(ctx context.Context, id string, p TaskPatch, actor string) (*Task, error) {
	var sets []string
	var args []any
	args = append(args, id)
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if p.Title != nil {
		set("title", *p.Title)
	}
	if p.Spec != nil {
		set("spec", *p.Spec)
	}
	if p.ColumnKey != nil {
		set("column_key", *p.ColumnKey)
	}
	if p.Position != nil {
		set("position", *p.Position)
	}
	if p.Priority != nil {
		set("priority", *p.Priority)
	}
	if p.Estimate != nil {
		set("estimate", *p.Estimate)
	}
	if p.RoleRequired != nil {
		set("role_required", nilIfEmpty(*p.RoleRequired))
	}
	if p.AssigneeID != nil {
		set("assignee_id", nilIfEmpty(*p.AssigneeID))
	}
	if p.Acceptance != nil {
		set("acceptance", *p.Acceptance)
	}
	if p.Labels != nil {
		set("labels", *p.Labels)
	}
	if p.DependsOn != nil {
		set("depends_on", *p.DependsOn)
	}
	if p.Close != nil {
		if *p.Close {
			sets = append(sets, "closed_at=now()")
		} else {
			sets = append(sets, "closed_at=NULL")
		}
	}
	if len(sets) == 0 {
		return s.GetTask(ctx, id)
	}
	if p.ColumnKey != nil {
		if err := s.checkWIP(ctx, id, *p.ColumnKey); err != nil {
			return nil, err
		}
	}
	sets = append(sets, "updated_at=now()")

	updated, err := selectOne[Task](ctx, s,
		`UPDATE tasks SET `+strings.Join(sets, ", ")+` WHERE id=$1 RETURNING `+taskCols, args...)
	if err != nil {
		return nil, err
	}
	kind := "updated"
	data := JSONMap{}
	if p.ColumnKey != nil {
		kind, data["column"] = "moved", *p.ColumnKey
	}
	if p.AssigneeID != nil {
		kind, data["assignee"] = "assigned", *p.AssigneeID
	}
	if p.Close != nil && *p.Close {
		kind = "closed"
	}
	_ = s.AddTaskEvent(ctx, id, actor, kind, data)
	return updated, nil
}

// ErrWIPExceeded is returned instead of silently letting a column overflow —
// a WIP limit nobody enforces is decoration.
type ErrWIPExceeded struct {
	Column string
	Limit  int
}

func (e ErrWIPExceeded) Error() string {
	return fmt.Sprintf("column %q is at its WIP limit of %d; finish or move something out first",
		e.Column, e.Limit)
}

// checkWIP counts the destination column, excluding the task being moved so a
// move within the same column never trips its own limit.
func (s *Store) checkWIP(ctx context.Context, taskID, column string) error {
	var limit, current int
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT (c->>'wip')::int
			FROM boards b, jsonb_array_elements(b.columns) c
			WHERE b.id = t.board_id AND c->>'key' = $2
		), 0),
		(SELECT count(*) FROM tasks x
		 WHERE x.board_id = t.board_id AND x.column_key = $2
		   AND x.closed_at IS NULL AND x.id <> t.id)
		FROM tasks t WHERE t.id = $1`, taskID, column).Scan(&limit, &current)
	if err != nil {
		return err
	}
	if limit > 0 && current >= limit {
		return ErrWIPExceeded{Column: column, Limit: limit}
	}
	return nil
}

// UnmetDependencies returns the titles of the task's still-open prerequisites.
// Dispatching past them is how a board produces work in the wrong order.
func (s *Store) UnmetDependencies(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.seq || ' ' || d.title
		FROM tasks t
		JOIN tasks d ON d.id = ANY(t.depends_on)
		WHERE t.id = $1 AND d.closed_at IS NULL AND d.column_key <> 'done'
		ORDER BY d.seq`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ColumnCounts returns open task counts keyed by column, so a WIP limit can be
// read before it is hit rather than discovered by a rejected move.
func (s *Store) ColumnCounts(ctx context.Context, boardID string) (map[string]int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT column_key, count(*)::int FROM tasks
		WHERE board_id=$1 AND closed_at IS NULL GROUP BY column_key`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// WouldCycle reports whether giving taskID the prerequisites in next puts it in
// a dependency cycle. Every task in such a cycle waits on itself forever and
// UnmetDependencies would report the block without ever explaining it, so this
// is checked before the write rather than diagnosed after.
func (s *Store) WouldCycle(ctx context.Context, taskID string, next []string) (bool, error) {
	if len(next) == 0 {
		return false, nil
	}
	var cyclic bool
	err := s.Pool.QueryRow(ctx, `
		WITH RECURSIVE reach(id) AS (
			SELECT unnest($2::uuid[])
			UNION
			SELECT unnest(t.depends_on) FROM tasks t JOIN reach ON t.id = reach.id
		)
		SELECT EXISTS (SELECT 1 FROM reach WHERE id = $1)`, taskID, next).Scan(&cyclic)
	return cyclic, err
}

// StalledTasks surfaces work that has sat untouched, for the sprint manager.
func (s *Store) StalledTasks(ctx context.Context, boardID string, idle time.Duration) ([]Task, error) {
	return selectAll[Task](ctx, s, `
		SELECT `+taskCols+` FROM tasks
		WHERE board_id=$1 AND closed_at IS NULL
		  AND column_key NOT IN ('backlog','done')
		  AND updated_at < now() - $2::interval
		ORDER BY updated_at`, boardID, idle.String())
}

// ClaimTask assigns an unassigned task to the agent atomically. Two agents
// racing on the same task: exactly one wins, the loser gets ErrNotFound.
func (s *Store) ClaimTask(ctx context.Context, taskID, agentID string) (*Task, error) {
	t, err := selectOne[Task](ctx, s, `
		UPDATE tasks SET assignee_id=$2, column_key='in_progress', updated_at=now()
		WHERE id=$1 AND assignee_id IS NULL AND closed_at IS NULL
		RETURNING `+taskCols, taskID, agentID)
	if err != nil {
		return nil, err
	}
	_ = s.AddTaskEvent(ctx, taskID, "agent:"+agentID, "assigned", JSONMap{"assignee": agentID})
	return t, nil
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	return err
}

func (s *Store) AddTaskEvent(ctx context.Context, taskID, actor, kind string, data JSONMap) error {
	if data == nil {
		data = JSONMap{}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO task_events (task_id, actor, kind, data) VALUES ($1,$2,$3,$4)`,
		taskID, orDefault(actor, "system"), kind, data)
	return err
}

func (s *Store) ListTaskEvents(ctx context.Context, taskID string) ([]TaskEvent, error) {
	return selectAll[TaskEvent](ctx, s,
		`SELECT id, task_id, actor, kind, data, created_at FROM task_events
		 WHERE task_id=$1 ORDER BY id`, taskID)
}

func nilIfZero(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
