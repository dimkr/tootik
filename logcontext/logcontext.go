package logcontext

import (
	"context"
	"log/slog"
)

type (
	keyType int

	handler struct {
		inner slog.Handler
	}
)

var key keyType

func New(ctx context.Context, args ...any) context.Context {
	return context.WithValue(ctx, key, args)
}

func (h handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	if v := ctx.Value(key); v != nil {
		r.Add(v.([]any)...)
	}
	return h.inner.Handle(ctx, r)
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{h.inner.WithAttrs(attrs)}
}

func (h handler) WithGroup(name string) slog.Handler {
	return &handler{h.inner.WithGroup(name)}
}

func Wrap(inner slog.Handler) slog.Handler {
	return &handler{}
}
