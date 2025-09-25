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

package fed

import "net/http"

func (l *Listener) handleIndex(w http.ResponseWriter, r *http.Request) {
	// this is how PieFed fetches the instance actor to detect its inbox and use it as a shared inbox for this instance
	if accept := r.Header.Get("Accept"); accept == "application/activity+json" || accept == `application/ld+json; profile="https://www.w3.org/ns/activitystreams"` {
		l.doHandleUser(w, r, "nobody")
		return
	}

	w.Header().Set("Location", "gemini://"+l.Domain)
	w.WriteHeader(http.StatusMovedPermanently)
}
