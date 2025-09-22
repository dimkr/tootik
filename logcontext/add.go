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

import "context"

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
