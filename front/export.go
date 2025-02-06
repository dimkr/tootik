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

package front

import (
	"bufio"
	"encoding/csv"

	"github.com/dimkr/tootik/front/text"
)

const (
	csvBufferSize = 32 * 1024
	maxCsvRows    = 100
	maxOffset     = 1000
)

func (h *Handler) export(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	if offset >= maxOffset {
		w.Statusf(40, "Offset must be <%d", maxOffset)
		return
	}

	output := csv.NewWriter(bufio.NewWriterSize(w, csvBufferSize))

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select activity->>'$.id', datetime(inserted, 'unixepoch'), activity from outbox
		where
			activity->>'$.actor' = ?
		order by inserted desc
		limit ?
		offset ?
		`,
		r.User.ID,
		maxCsvRows,
		offset,
	)
	if err != nil {
		r.Log.Warn("Failed to fetch activities", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	w.Status(20, "text/csv")

	var fields [3]string
	for rows.Next() {
		if err := rows.Scan(&fields[0], &fields[1], &fields[2]); err != nil {
			r.Log.Warn("Failed to scan activity", "error", err)
			continue
		}

		if err := output.Write(fields[:]); err != nil {
			r.Log.Warn("Failed to write a line", "error", err)
			return
		}
	}

	output.Flush()
	if err := output.Error(); err != nil {
		r.Log.Warn("Failed to flush output", "error", err)
	}
}
