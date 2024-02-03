/*
Copyright 2024 Dima Krasner

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
	"fmt"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
	"net/url"
	"strings"
	"time"
)

func (h *Handler) move(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.User.MovedTo != "" {
		r.Log.Warn("User cannot be moved again", "movedTo", r.User.MovedTo)
		w.Status(40, "Already moved to "+r.User.MovedTo)
		return
	}

	now := time.Now()

	if (r.User.Updated != nil && now.Sub(r.User.Updated.Time) < h.Config.MinActorEditInterval) || (r.User.Updated == nil && now.Sub(r.User.Published.Time) < h.Config.MinActorEditInterval) {
		r.Log.Warn("Throttled request to move account")
		w.Status(40, "Please try again later")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Target (name@domain)")
		return
	}

	target, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Warn("Failed to decode move target", "query", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	tokens := strings.SplitN(target, "@", 3)
	if len(tokens) != 2 {
		r.Log.Warn("Target is invalid", "target", target)
		w.Status(40, "Bad input")
		return
	}

	actor, err := r.Resolve(fmt.Sprintf("https://%s/user/%s", tokens[1], tokens[0]), false)
	if err != nil {
		r.Log.Warn("Failed to resolve target", "target", target, "error", err)
		w.Status(40, "Failed to resolve "+target)
		return
	}

	if !r.User.AlsoKnownAs.Contains(actor.ID) {
		r.Log.Warn("Move source is not an alias for target", "target", target)
		w.Statusf(40, "%s is not an alias for %s", r.User.ID, actor.ID)
		return
	}

	if !actor.AlsoKnownAs.Contains(r.User.ID) {
		r.Log.Warn("Move target is not an alias for source", "target", target)
		w.Statusf(40, "%s is not an alias for %s", actor.ID, r.User.ID)
		return
	}

	if err := outbox.Move(r.Context, r.DB, r.Handler.Domain, r.User, actor.ID); err != nil {
		r.Log.Error("Failed to move user", "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/outbox/" + strings.TrimPrefix(actor.ID, "https://"))
}
