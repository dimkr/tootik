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

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

const (
	csvBufferSize = 32 * 1024
	csvRows       = 200
)

var csvHeader = []string{"ID", "Type", "Inserted", "Activity"}

func (h *Handler) export(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	output := csv.NewWriter(bufio.NewWriterSize(w, csvBufferSize))

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select id, activity->>'$.type', datetime(inserted, 'unixepoch'), json(activity) from outbox
		where
			activity->>'$.actor' in (select id from persons where id = ?)
		order by inserted desc
		limit ?
		`,
		ap.Canonical(r.User.ID),
		csvRows,
	)
	if err != nil {
		r.Log.Warn("Failed to fetch activities", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	w.Status(20, "text/csv")

	if err := output.Write(csvHeader); err != nil {
		r.Log.Warn("Failed to write header", "error", err)
		return
	}

	var fields [4]string
	for rows.Next() {
		if err := rows.Scan(&fields[0], &fields[1], &fields[2], &fields[3]); err != nil {
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
