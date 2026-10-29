package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LogEntry struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	EventType string     `json:"event_type"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}

type LogQuery struct {
	EventType string
	UserID    *uuid.UUID
	From      *time.Time
	To        *time.Time
	Cursor    *time.Time
	Limit     int
}

func ListLogs(ctx context.Context, pool *pgxpool.Pool, q LogQuery) ([]LogEntry, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, event_type, message, created_at
		FROM system_logs
		WHERE ($1::text IS NULL OR $1 = '' OR event_type = $1)
		  AND ($2::uuid IS NULL OR user_id = $2)
		  AND ($3::timestamptz IS NULL OR created_at >= $3)
		  AND ($4::timestamptz IS NULL OR created_at <= $4)
		  AND ($5::timestamptz IS NULL OR created_at < $5)
		ORDER BY created_at DESC
		LIMIT $6
	`, nullStr(q.EventType), q.UserID, q.From, q.To, q.Cursor, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.EventType, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []LogEntry{}
	}
	return out, rows.Err()
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
