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
	"github.com/dimkr/tootik/cfg"
	"log/slog"
	"os"
)

func new(fields ...slog.Attr) *Logger {
	logLevel := slog.Level(cfg.LogLevel)

	var opts slog.HandlerOptions
	if logLevel == slog.LevelDebug {
		opts.AddSource = true
	}

	return &Logger{
		Logger: slog.New(slog.NewJSONHandler(os.Stderr, &opts).WithAttrs(fields)),
	}
}
