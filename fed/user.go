/*
Copyright 2023, 2024 Dima Krasner

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

package fed

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func (l *Listener) handleUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")

	l.Log.Info("Looking up user", "name", name)

	var actorID, actorString string
	if err := l.DB.QueryRowContext(r.Context(), `select id, actor from persons where actor->>'preferredUsername' = ? and host = ?`, name, l.Domain).Scan(&actorID, &actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		l.Log.Info("Notifying about deleted user", "id", actorID)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// redirect browsers to the outbox page over Gemini
	if shouldRedirect(r) {
		outbox := fmt.Sprintf("gemini://%s/outbox/%s", l.Domain, strings.TrimPrefix(actorID, "https://"))
		l.Log.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write([]byte(actorString))
}
