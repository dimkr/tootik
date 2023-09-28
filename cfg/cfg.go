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

package cfg

import (
	"flag"
	"log/slog"
)

const (
	MaxPostsLength      = 200
	MaxResolverRequests = 16
)

var (
	Domain   string
	LogLevel int
)

func init() {
	flag.StringVar(&Domain, "domain", "localhost.localdomain", "Domain name")
	flag.IntVar(&LogLevel, "loglevel", int(slog.LevelInfo), "Logging verbosity")
}
