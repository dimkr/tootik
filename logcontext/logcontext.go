/*
Copyright 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// Add adds log fields to a [context.Context].
//
// Arguments should be in the same format as [slog.Logger.Log].
//
// Use [NewHandler] to obtain a [slog.Handler] that logs these fields.
func Add(ctx context.Context, args ...any) context.Context {
	if v := ctx.Value(key); v != nil {
		return context.WithValue(ctx, key, append(v.([]any), args...))
	}

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

// NewHandler returns a [slog.Handler] that adds the log fields passed to [Add].
func NewHandler(inner slog.Handler) slog.Handler {
	return &handler{inner: inner}
}
