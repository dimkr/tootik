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

var def = new()

func Default() *Logger {
	return def
}

func WithField(k string, v any) Entry {
	return def.WithField(k, v)
}

func WithFields(fields Fields) Entry {
	return def.WithFields(fields)
}

func WithError(err error) Entry {
	return def.WithError(err)
}

func With(args ...any) *Logger {
	return &Logger{def.With(args...)}
}

func Fatal(err error) {
	def.Fatal(err)
}

func Warn(msg string) {
	def.Warn(msg)
}

func Info(msg string) {
	def.Info(msg)
}
