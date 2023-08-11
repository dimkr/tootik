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

import (
	"fmt"
	"log/slog"
)

type Entry interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	WithError(err error) Entry
	Warnf(fmt string, args ...any)
}

type entry struct {
	Logger *Logger
	Fields map[string]slog.Attr
}

func (e *entry) getValues() []any {
	values := make([]any, 0, len(e.Fields))
	for _, v := range e.Fields {
		values = append(values, v)
	}
	return values
}

func (e *entry) Error(msg string, args ...any) {
	e.Logger.Error(msg, append(e.getValues(), args...)...)
}

func (e *entry) Warn(msg string, args ...any) {
	e.Logger.Warn(msg, append(e.getValues(), args...)...)
}

func (e *entry) Info(msg string, args ...any) {
	e.Logger.Info(msg, append(e.getValues(), args...)...)
}

func (e *entry) Debug(msg string, args ...any) {
	e.Logger.Debug(msg, append(e.getValues(), args...)...)
}

func (e *entry) WithError(err error) Entry {
	e.Fields["error"] = slog.Any("error", err)
	return e
}

func (e *entry) Warnf(f string, args ...any) {
	e.Logger.Warn(fmt.Sprintf(f, args...), e.getValues()...)
}
