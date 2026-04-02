//go:build (cgo && sqlite_mattn && !sqlite_modernc && !sqlite_ncruces) || (cgo && !sqlite_mattn && !sqlite_modernc && !sqlite_ncruces)

/*
Copyright 2026 Dima Krasner

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

package sqlite

import _ "github.com/mattn/go-sqlite3"

const (
	DriverName = "sqlite3"

	Scheme         = ""
	DefaultOptions = "_journal_mode=WAL&_synchronous=1&_busy_timeout=5000&_txlock=immediate"
	JournalModeWAL = "_journal_mode=WAL"
)
