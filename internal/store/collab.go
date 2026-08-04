package store

import "context"

const meetingCols = `id, kind, title, agenda, state, round, max_rounds, participants,
	facilitator, board_id, decision, created_at, closed_at`

func (s *Store) ListMeetings(ctx context.Context, openOnly bool) ([]Meeting, error) {
	sql := `SELECT ` + meetingCols + ` FROM meetings`
	if openOnly {
		sql += ` WHERE closed_at IS NULL`
	}
	return selectAll[Meeting](ctx, s, sql+` ORDER BY created_at DESC`)
}

func (s *Store) GetMeeting(ctx context.Context, id string) (*Meeting, error) {
	return selectOne[Meeting](ctx, s, `SELECT `+meetingCols+` FROM meetings WHERE id=$1`, id)
}

func (s *Store) CreateMeeting(ctx context.Context, m Meeting) (*Meeting, error) {
	return selectOne[Meeting](ctx, s, `
		INSERT INTO meetings (kind, title, agenda, participants, max_rounds, board_id, facilitator)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+meetingCols,
		orDefault(m.Kind, "design"), m.Title, m.Agenda, nonNil(m.Participants),
		orDefaultInt(m.MaxRounds, 3), m.BoardID, orDefault(m.Facilitator, "brain"))
}

func (s *Store) SetMeetingState(ctx context.Context, id, state string, round int) error {
	_, err := s.Pool.Exec(ctx, `UPDATE meetings SET state=$2, round=$3 WHERE id=$1`, id, state, round)
	return err
}

func (s *Store) CloseMeeting(ctx context.Context, id, decision string) (*Meeting, error) {
	return selectOne[Meeting](ctx, s, `
		UPDATE meetings SET state='closed', decision=$2, closed_at=now()
		WHERE id=$1 RETURNING `+meetingCols, id, decision)
}

const msgCols = `id, meeting_id, agent_id, speaker, round, content, refs, created_at`

func (s *Store) PostMessage(ctx context.Context, m MeetingMessage) (*MeetingMessage, error) {
	if m.Refs == nil {
		m.Refs = JSONMap{}
	}
	return selectOne[MeetingMessage](ctx, s, `
		INSERT INTO meeting_messages (meeting_id, agent_id, speaker, round, content, refs)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+msgCols,
		m.MeetingID, m.AgentID, m.Speaker, m.Round, m.Content, m.Refs)
}

func (s *Store) ListMessages(ctx context.Context, meetingID string, sinceID int64) ([]MeetingMessage, error) {
	return selectAll[MeetingMessage](ctx, s,
		`SELECT `+msgCols+` FROM meeting_messages WHERE meeting_id=$1 AND id>$2 ORDER BY id`,
		meetingID, sinceID)
}
