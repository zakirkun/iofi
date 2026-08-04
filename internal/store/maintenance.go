package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// RecoverStaleRuns runs at boot. A process that died mid-run leaves rows saying
// "running" and agents saying "busy" forever, and PickAgentForRole then treats
// every agent as full. Nothing can resume those processes, so they are failed.
func (s *Store) RecoverStaleRuns(ctx context.Context) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE runs SET state='failed', finished_at=now(),
		       error=COALESCE(NULLIF(error,''), 'iofi restarted while this run was in flight')
		WHERE state IN ('queued','running')`)
	if err != nil {
		return 0, err
	}
	if _, err := s.Pool.Exec(ctx,
		`UPDATE agents SET status='idle', updated_at=now() WHERE status IN ('busy','error')`); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// PruneRunEvents keeps the event log from growing without bound. Only events of
// finished runs are dropped, so a live run is never truncated under itself.
func (s *Store) PruneRunEvents(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM run_events re
		USING runs r
		WHERE re.run_id = r.id
		  AND r.state IN ('done','failed','cancelled')
		  AND re.at < now() - $1::interval`, olderThan.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---------------------------------------------------------------- settings

func (s *Store) GetSetting(ctx context.Context, key string, dst any) error {
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func (s *Store) SetSetting(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1,$2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, raw)
	return err
}

// TokenPrice is USD per one million tokens.
type TokenPrice struct {
	In  float64 `json:"in"`
	Out float64 `json:"out"`
}

const PricesKey = "token_prices"

// Prices lets iofi cost runs from tools that report tokens but not money.
// Claude Code reports its own cost and is left alone.
func (s *Store) Prices(ctx context.Context) map[string]TokenPrice {
	out := map[string]TokenPrice{}
	if err := s.GetSetting(ctx, PricesKey, &out); err != nil {
		return nil
	}
	return out
}

func (s *Store) SeedPrices(ctx context.Context) error {
	if err := s.GetSetting(ctx, PricesKey, &map[string]TokenPrice{}); err == nil {
		return nil // already set; never overwrite the operator's numbers
	}
	return s.SetSetting(ctx, PricesKey, map[string]TokenPrice{
		"gpt-4o":                 {In: 2.50, Out: 10.00},
		"gpt-4o-mini":            {In: 0.15, Out: 0.60},
		"claude-sonnet":          {In: 3.00, Out: 15.00},
		"text-embedding-3-small": {In: 0.02, Out: 0},
	})
}

// CostOf prices a token count, falling back to a "default" entry so an unknown
// model is reported as an estimate rather than as free.
func CostOf(prices map[string]TokenPrice, model string, in, out int64) float64 {
	p, ok := prices[model]
	if !ok {
		if p, ok = prices["default"]; !ok {
			return 0
		}
	}
	return (float64(in)*p.In + float64(out)*p.Out) / 1_000_000
}

// ---------------------------------------------------------------- embeddings

// CheckEmbedDim compares the configured embedding size against the actual
// column. A mismatch fails every insert at runtime with an opaque error, so it
// is reported once at boot with the fix.
func (s *Store) CheckEmbedDim(ctx context.Context, want int) error {
	var dim int
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(atttypmod, 0) FROM pg_attribute
		WHERE attrelid = 'memories'::regclass AND attname = 'embedding'`).Scan(&dim)
	if err != nil {
		return err
	}
	if dim == want {
		return nil
	}
	return fmt.Errorf(
		"memories.embedding is vector(%d) but IOFI_EMBED_DIM is %d; "+
			"either set IOFI_EMBED_DIM=%d or re-embed with:\n"+
			"  UPDATE memories SET embedding = NULL;\n"+
			"  ALTER TABLE memories ALTER COLUMN embedding TYPE vector(%d);",
		dim, want, dim, want)
}

// Janitor runs the periodic cleanups. One goroutine, started from main.
func (s *Store) Janitor(ctx context.Context, every time.Duration, retention time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.PruneRunEvents(ctx, retention); err != nil {
				slog.Warn("prune run events", "err", err)
			} else if n > 0 {
				slog.Info("pruned run events", "rows", n)
			}
			if _, err := s.Pool.Exec(ctx,
				`DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at < now()`); err != nil {
				slog.Warn("prune memories", "err", err)
			}
		}
	}
}
