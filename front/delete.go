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

package front

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/text"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/delete/[0-9a-f]{64}`)] = delete
}

func delete(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := filepath.Base(r.URL.Path)

	var noteString string
	if err := r.QueryRow(`select object from notes where hash = ? and author = ?`, hash, r.User.ID).Scan(&noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to delete a non-existing post", "hash", hash, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch post to delete", "hash", hash, "error", err)
		w.Error()
		return
	}

	var note ap.Object
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Warn("Failed to unmarshal post to delete", "hash", hash, "error", err)
		w.Error()
		return
	}

	if err := fed.Delete(r.Context, r.DB, &note); err != nil {
		r.Log.Error("Failed to delete post", "note", note.ID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(r.User.ID)))
}
