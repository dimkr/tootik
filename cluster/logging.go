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

package cluster

import (
	"log/slog"
	"os"

	"github.com/dimkr/tootik/logcontext"
)

func init() {
	opts := slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}

	slog.SetDefault(slog.New(logcontext.NewHandler(slog.NewJSONHandler(os.Stderr, &opts))))
}
