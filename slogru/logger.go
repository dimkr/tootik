/*
Copyright 2023 Dima Krasner

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

package slogru

import "log/slog"

type Logger struct {
	*slog.Logger
}

func (l *Logger) WithField(k string, v any) Entry {
	return &Logger{l.With(k, v)}
}

func (l *Logger) WithFields(fields Fields) Entry {
	attrs := make([]slog.Attr, len(fields))
	i := 0
	for k, v := range fields {
		attrs[i] = slog.Any(k, v)
		i++
	}
	return &Logger{l.With(attrs)}
}

func (l *Logger) WithError(err error) Entry {
	return &Logger{l.With("error", err)}
}

func (l *Logger) Fatal(err error) {
	l.WithError(err).Error("Fatal")
	panic(err)
}

func (l *Logger) Warnf(fmt string, args ...any) {
	l.Warn(fmt, args...)
}
