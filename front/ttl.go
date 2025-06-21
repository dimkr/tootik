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
	"database/sql"
	"net/url"
	"strconv"

	"github.com/dimkr/tootik/front/text"
)

const maxDays = 365 * 2

func (h *Handler) ttl(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	var ttl sql.NullInt32

	if r.URL.RawQuery == "" {
		if err := h.DB.QueryRowContext(
			r.Context,
			`select ttl from persons where id = ?`,
			r.User.ID,
		).Scan(&ttl); err != nil {
			r.Log.Warn("Failed to fetch TTL", "error", err)
			w.Error()
			return
		}
	} else {
		s, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			r.Log.Warn("Failed to decode number of days", "query", r.URL.RawQuery, "error", err)
			w.Status(40, "Bad input")
			return
		}

		days, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			r.Log.Warn("Failed to parse number of days", "query", r.URL.RawQuery, "error", err)
			w.Status(40, "Bad input")
			return
		}

		if days < 0 || days > maxDays {
			r.Log.Warn("Failed to parse number of days", "query", r.URL.RawQuery, "error", err)
			w.Status(40, "Bad input")
			return
		}

		if days > 0 {
			if _, err := h.DB.ExecContext(
				r.Context,
				`update persons set ttl = ? where id = ?`,
				days,
				r.User.ID,
			); err != nil {
				r.Log.Error("Failed to set TTL", "error", err)
				w.Error()
				return
			}

			ttl.Valid = true
			ttl.Int32 = int32(days)
		} else if _, err := h.DB.ExecContext(
			r.Context,
			`update persons set ttl = null where id = ?`,
			r.User.ID,
		); err != nil {
			r.Log.Error("Failed to clear TTL", "error", err)
			w.Error()
			return
		}
	}

	w.OK()
	w.Title("⏳ Post Deletion Policy")

	if ttl.Valid {
		switch ttl.Int32 {
		case 7:
			w.Text("Current setting: posts are deleted after a week.")
		case 14:
			w.Text("Current setting: posts are deleted after two weeks.")
		case 30:
			w.Text("Current setting: posts are deleted after a month.")
		case 60:
			w.Text("Current setting: posts are deleted after two months.")
		case 180:
			w.Text("Current setting: posts are deleted after six months.")
		case 365:
			w.Text("Current setting: posts are deleted after a year.")
		case 730:
			w.Text("Current setting: posts are deleted after two years.")
		default:
			w.Textf("Current setting: posts are deleted after %d days.", ttl.Int32)
		}
	} else {
		w.Text("Current setting: old posts are not deleted automatically.")
	}
	w.Empty()

	if !ttl.Valid || ttl.Int32 != 7 {
		w.Link("/users/ttl?7", "After a week")
	}

	if !ttl.Valid || ttl.Int32 != 14 {
		w.Link("/users/ttl?14", "After two weeks")
	}

	if !ttl.Valid || ttl.Int32 != 30 {
		w.Link("/users/ttl?30", "After a month")
	}

	if !ttl.Valid || ttl.Int32 != 60 {
		w.Link("/users/ttl?60", "After two months")
	}

	if !ttl.Valid || ttl.Int32 != 180 {
		w.Link("/users/ttl?180", "After six months")
	}

	if !ttl.Valid || ttl.Int32 != 365 {
		w.Link("/users/ttl?365", "After a year")
	}

	if !ttl.Valid || ttl.Int32 != 730 {
		w.Link("/users/ttl?730", "After two years")
	}

	if ttl.Valid {
		w.Link("/users/ttl?0", "Never")
	}
}
