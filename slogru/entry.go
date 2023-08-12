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

type entryWithField struct {
	Logger *slog.Logger
	Field  slog.Attr
}

type entryWithFields struct {
	Logger *slog.Logger
	Fields map[string]slog.Attr
}

func (e entryWithField) Error(msg string, args ...any) {
	e.Logger.Error(msg, append(args, e.Field)...)
}

func (e entryWithField) Warn(msg string, args ...any) {
	e.Logger.Warn(msg, append(args, e.Field)...)
}

func (e entryWithField) Info(msg string, args ...any) {
	e.Logger.Info(msg, append(args, e.Field)...)
}

func (e entryWithField) Debug(msg string, args ...any) {
	e.Logger.Debug(msg, append(args, e.Field)...)
}

func (e entryWithField) WithError(err error) Entry {
	if e.Field.Key == "error" {
		e.Field.Value = slog.AnyValue(err)
		return e
	}

	return &entryWithFields{Logger: e.Logger, Fields: map[string]slog.Attr{e.Field.Key: e.Field, "error": slog.Any("error", err)}}
}

func (e entryWithField) Warnf(f string, args ...any) {
	e.Logger.Warn(fmt.Sprintf(f, args...), e.Field)
}

func (e *entryWithFields) getValues() []any {
	values := make([]any, 0, len(e.Fields))
	for _, v := range e.Fields {
		values = append(values, v)
	}
	return values
}

func (e *entryWithFields) Error(msg string, args ...any) {
	e.Logger.Error(msg, append(e.getValues(), args...)...)
}

func (e *entryWithFields) Warn(msg string, args ...any) {
	e.Logger.Warn(msg, append(e.getValues(), args...)...)
}

func (e *entryWithFields) Info(msg string, args ...any) {
	e.Logger.Info(msg, append(e.getValues(), args...)...)
}

func (e *entryWithFields) Debug(msg string, args ...any) {
	e.Logger.Debug(msg, append(e.getValues(), args...)...)
}

func (e *entryWithFields) WithError(err error) Entry {
	e.Fields["error"] = slog.Any("error", err)
	return e
}

func (e *entryWithFields) Warnf(f string, args ...any) {
	e.Logger.Warn(fmt.Sprintf(f, args...), e.getValues()...)
}
