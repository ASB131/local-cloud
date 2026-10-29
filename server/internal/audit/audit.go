package audit

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Writer struct {
	pool *pgxpool.Pool
}

func NewWriter(pool *pgxpool.Pool) *Writer {
	return &Writer{pool: pool}
}

func (w *Writer) Write(ctx context.Context, userID *uuid.UUID, eventType, message string, ip net.IP) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var ipArg any
	if ip != nil {
		ipArg = ip.String()
	}
	_, err = w.pool.Exec(ctx, `
		INSERT INTO system_logs (id, user_id, event_type, message, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, userID, eventType, message, ipArg, time.Now().UTC())
	return err
}
