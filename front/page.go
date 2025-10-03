/*
Copyright 2023 - 2025 Dima Krasner

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
	"fmt"
	"net/url"
	"strconv"

	"github.com/dimkr/tootik/front/text"
)

func getOffset(requestUrl *url.URL) (int, error) {
	if requestUrl.RawQuery == "" {
		return 0, nil
	}

	query, err := url.QueryUnescape(requestUrl.RawQuery)
	if err != nil {
		return 0, err
	}

	offset, err := strconv.ParseInt(query, 10, 32)
	if err != nil {
		return 0, err
	}

	return int(offset), nil
}

func (h *Handler) showFeedPage(w text.Writer, r *Request, title string, query func(int) (*sql.Rows, error), printDaySeparators bool) {
	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	if offset > h.Config.MaxOffset {
		r.Log.Warn("Offset is too big", "offset", offset)
		w.Statusf(40, "Offset must be <= %d", h.Config.MaxOffset)
		return
	}

	rows, err := query(offset)
	if err != nil {
		r.Log.Warn("Failed to fetch posts", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	w.OK()
	if offset > 0 {
		w.Titlef("%s (%d-%d)", title, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Title(title)
	}

	count := h.PrintNotes(w, r, rows, true, printDaySeparators, "No posts.")
	rows.Close()

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Empty()
		w.Subtitle("Navigation")
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage && offset+h.Config.PostsPerPage <= h.Config.MaxOffset {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}
