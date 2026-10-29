package httpapi

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyAdmin
)

func withUser(ctx context.Context, userID uuid.UUID, admin bool) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	return context.WithValue(ctx, ctxKeyAdmin, admin)
}

func userFromContext(ctx context.Context) (uuid.UUID, bool, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil, false, false
	}
	admin, _ := ctx.Value(ctxKeyAdmin).(bool)
	return id, true, admin
}
